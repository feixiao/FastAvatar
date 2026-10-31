#include "LockFreeRingBuffer.hpp"
#include "CpuAffinity.hpp"
#include "StreamingSentenceChunker.hpp"
#include "WatchdogAndRepetition.hpp"
#include "ThreeTierMemory.hpp"
#include "FullDuplexAudioIO.hpp"
#include "AudioAgentPipeline.hpp"
#include "MockCloudLlmEngine.hpp"
#include "IntentArbiter.hpp"
#include <iostream>
#include <chrono>
#include <vector>
#include <iomanip>
#include <atomic>
#include <thread>
#include <mutex>
#include <condition_variable>

using namespace audio_agent;

static double BenchmarkRingBuffer() {
    constexpr size_t NUM_OPS = 100000;
    LockFreeSPSCRingBuffer<float> rb(4096);
    std::vector<float> in_buf(256, 0.5f);
    std::vector<float> out_buf(256, 0.0f);
    volatile float sink = 0.0f;
    auto start = std::chrono::high_resolution_clock::now();
    for (size_t i = 0; i < NUM_OPS; ++i) {
        rb.write(in_buf.data(), in_buf.size());
        rb.read(out_buf.data(), in_buf.size());
        sink += out_buf[0];
    }
    auto end = std::chrono::high_resolution_clock::now();
    (void)sink;
    auto elapsed_ns = std::chrono::duration_cast<std::chrono::nanoseconds>(end - start).count();
    return static_cast<double>(elapsed_ns) / static_cast<double>(NUM_OPS);
}

static double BenchmarkIntentArbiter() {
    IntentArbiter arbiter;
    const std::vector<std::string> queries = {
        "把空调调到二十四度",
        "稍微调低一点温度",
        "帮我导航到西湖断桥",
        "播放周杰伦的七里香",
        "暂停播放音乐",
        "调大一点声音",
        "帮我定一个明天早晨七点半的闹钟",
        "给张三打个电话",
        "今天杭州的天气怎么样",
        "今天星期几啊"
    };

    constexpr size_t ITERATIONS = 5000;
    auto start = std::chrono::high_resolution_clock::now();
    for (size_t i = 0; i < ITERATIONS; ++i) {
        const auto& q = queries[i % queries.size()];
        auto res = arbiter.Arbitrate(q);
        (void)res;
    }
    auto end = std::chrono::high_resolution_clock::now();
    auto elapsed_us = std::chrono::duration_cast<std::chrono::microseconds>(end - start).count();
    return static_cast<double>(elapsed_us) / static_cast<double>(ITERATIONS);
}

static double BenchmarkSentenceChunker() {
    std::vector<std::string> emitted;
    SentenceChunkerCallbacks callbacks;
    callbacks.on_sentence_ready = [&](const std::string& s) { emitted.push_back(s); };
    callbacks.on_tool_call = [](const std::string&) {};
    callbacks.is_aborted = []() { return false; };

    StreamingSentenceChunker chunker(callbacks);
    std::vector<std::string> tokens = {
        "好", "的", "，", "马", "上", "为", "您", "处", "理", "。",
        "今", "天", "天", "气", "晴", "朗", "，", "气", "温", "二", "十", "二", "度", "。",
        "如", "需", "帮", "助", "请", "随", "时", "唤", "醒", "我", "。"
    };

    constexpr size_t ITERATIONS = 1000;
    auto start = std::chrono::high_resolution_clock::now();
    for (size_t iter = 0; iter < ITERATIONS; ++iter) {
        chunker.Reset();
        for (const auto& tok : tokens) {
            chunker.sputn(tok.data(), static_cast<std::streamsize>(tok.size()));
        }
        chunker.Flush();
    }
    auto end = std::chrono::high_resolution_clock::now();
    (void)emitted.size();
    auto elapsed_us = std::chrono::duration_cast<std::chrono::microseconds>(end - start).count();
    size_t total_tokens = ITERATIONS * tokens.size();
    return static_cast<double>(elapsed_us) / static_cast<double>(total_tokens);
}

static double BenchmarkBargeIn() {
    constexpr size_t ITERATIONS = 1000;
    auto mem = std::make_shared<ThreeTierMemory>("系统提示词");
    LockFreeSPSCRingBuffer<float> spk_rb(4096);
    std::vector<float> audio(480, 0.1f);

    auto start = std::chrono::high_resolution_clock::now();
    for (size_t i = 0; i < ITERATIONS; ++i) {
        spk_rb.write(audio.data(), audio.size());
        mem->AddTurn("你好", "这是一段正在朗读中的语音长回复内容", false);

        std::atomic<bool> abort_flag{false};
        abort_flag.store(true, std::memory_order_release);
        spk_rb.clear();
        mem->RollbackLastAssistantTurn("这是一段正在朗读");
    }
    auto end = std::chrono::high_resolution_clock::now();
    (void)mem->GetActiveTurns().size();
    auto elapsed_us = std::chrono::duration_cast<std::chrono::microseconds>(end - start).count();
    return static_cast<double>(elapsed_us) / static_cast<double>(ITERATIONS);
}

class FastBenchTtsEngine : public ISpeechSynthesisEngine {
public:
    bool SynthesizeStreaming(const std::string&, AudioChunkCallback callback) override {
        std::vector<float> pcm(480, 0.0f);
        return callback(pcm.data(), static_cast<int32_t>(pcm.size()), 1.0f) != 0;
    }
    int32_t GetSampleRate() const override { return 24000; }
};

class FastBenchPerceptionEngine : public ISpeechPerceptionEngine {
public:
    bool AcceptAudioChunk(const float*, int32_t) override { return false; }
    bool IsEndpointDetected() override { return false; }
    std::string GetRecognizedText(bool) override { return ""; }
    void Reset() override {}
};

static double BenchmarkPipelineTurnaround() {
    constexpr size_t ITERATIONS = 5;

    auto perception = std::make_shared<FastBenchPerceptionEngine>();
    auto llm = std::make_shared<MockCloudLlmEngine>();
    auto tts = std::make_shared<FastBenchTtsEngine>();

    std::mutex cv_mtx;
    std::condition_variable cv;
    std::atomic<bool> turn_done{false};

    PipelineObserver observer;
    observer.on_state_changed = [&](AgentState new_state, AgentState old_state) {
        if (old_state != AgentState::IDLE && new_state == AgentState::IDLE) {
            {
                std::lock_guard<std::mutex> lock(cv_mtx);
                turn_done = true;
            }
            cv.notify_one();
        }
    };

    AudioAgentPipeline pipeline(perception, llm, tts, observer);
    pipeline.Start();

    auto start = std::chrono::high_resolution_clock::now();
    for (size_t i = 0; i < ITERATIONS; ++i) {
        {
            std::lock_guard<std::mutex> lock(cv_mtx);
            turn_done = false;
        }
        pipeline.SubmitUserQuery("今天天气怎么样");
        std::unique_lock<std::mutex> lock(cv_mtx);
        cv.wait_for(lock, std::chrono::milliseconds(200), [&]() { return turn_done.load(); });
    }
    auto end = std::chrono::high_resolution_clock::now();
    pipeline.Stop();

    auto elapsed_us = std::chrono::duration_cast<std::chrono::microseconds>(end - start).count();
    return static_cast<double>(elapsed_us) / static_cast<double>(ITERATIONS);
}

int main() {
    std::cout << std::fixed << std::setprecision(2);
    
    double rb_ns = BenchmarkRingBuffer();
    double arbiter_us = BenchmarkIntentArbiter();
    double chunker_us = BenchmarkSentenceChunker();
    double bargein_us = BenchmarkBargeIn();
    double pipeline_us = BenchmarkPipelineTurnaround();

    std::cout << "--- Benchmark Results ---\n";
    std::cout << "Ring Buffer latency: " << rb_ns << " ns/op\n";
    std::cout << "Intent Arbiter latency: " << arbiter_us << " us/query\n";
    std::cout << "Sentence Chunker latency: " << chunker_us << " us/token\n";
    std::cout << "Barge-in Interrupt latency: " << bargein_us << " us/interrupt\n";
    std::cout << "Pipeline Turnaround latency: " << pipeline_us << " us/turn\n";
    std::cout << "-------------------------\n";

    std::cout << "METRIC pipeline_latency_us=" << pipeline_us << "\n";
    std::cout << "METRIC barge_in_latency_us=" << bargein_us << "\n";
    std::cout << "METRIC chunker_latency_us=" << chunker_us << "\n";
    std::cout << "METRIC ring_buffer_ns=" << rb_ns << "\n";
    std::cout << "METRIC intent_arbiter_us=" << arbiter_us << "\n";

    return 0;
}
