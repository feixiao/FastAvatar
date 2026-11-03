#include "WatchdogAndRepetition.hpp"

namespace audio_agent {

TTFTWatchdog::TTFTWatchdog() {
    is_running_.store(true, std::memory_order_release);
    worker_thread_ = std::thread(&TTFTWatchdog::RunLoop, this);
}

TTFTWatchdog::~TTFTWatchdog() {
    Stop();
    {
        std::lock_guard<std::mutex> lock(mtx_);
        is_running_.store(false, std::memory_order_release);
    }
    cv_.notify_all();
    if (worker_thread_.joinable()) {
        try {
            worker_thread_.join();
        } catch (...) {}
    }
}

/**
 * @brief 启动首字延迟看门狗（TTFT Watchdog）
 * 
 * 复用持久化后台定时线程，避免单次查询频繁创建和销毁 OS 线程带来的系统调用开销。
 */
void TTFTWatchdog::Start(int timeout_ms, std::function<void()> on_timeout) {
    std::lock_guard<std::mutex> lock(mtx_);
    on_timeout_ = std::move(on_timeout);
    deadline_ = std::chrono::steady_clock::now() + std::chrono::milliseconds(timeout_ms);
    fed_.store(false, std::memory_order_release);
    is_armed_.store(true, std::memory_order_release);
    cv_.notify_all();
}

/**
 * @brief 喂狗操作：当第一句有效文本成功产出时解除定时器
 */
void TTFTWatchdog::Feed() {
    {
        std::lock_guard<std::mutex> lock(mtx_);
        fed_.store(true, std::memory_order_release);
        is_armed_.store(false, std::memory_order_release);
    }
    cv_.notify_all();
}

/**
 * @brief 停止看门狗
 */
void TTFTWatchdog::Stop() {
    {
        std::lock_guard<std::mutex> lock(mtx_);
        is_armed_.store(false, std::memory_order_release);
    }
    cv_.notify_all();
}

/**
 * @brief 看门狗持久化等待循环（运行于独立定时线程）
 */
void TTFTWatchdog::RunLoop() {
    std::unique_lock<std::mutex> lock(mtx_);
    while (is_running_.load(std::memory_order_acquire)) {
        if (!is_armed_.load(std::memory_order_acquire)) {
            cv_.wait(lock, [this]() {
                return !is_running_.load(std::memory_order_acquire) ||
                       is_armed_.load(std::memory_order_acquire);
            });
            continue;
        }

        auto deadline = deadline_;
        cv_.wait_until(lock, deadline, [this]() {
            return !is_running_.load(std::memory_order_acquire) ||
                   !is_armed_.load(std::memory_order_acquire) ||
                   fed_.load(std::memory_order_acquire);
        });

        if (!is_running_.load(std::memory_order_acquire)) {
            break;
        }

        if (fed_.load(std::memory_order_acquire) || !is_armed_.load(std::memory_order_acquire)) {
            is_armed_.store(false, std::memory_order_release);
            continue;
        }

        // 达到超时时间且未被喂狗
        if (std::chrono::steady_clock::now() >= deadline) {
            is_armed_.store(false, std::memory_order_release);
            auto cb = on_timeout_;
            lock.unlock(); // 触发外部回调前释放锁，防范死锁
            if (cb) {
                cb();
            }
            lock.lock();
        }
    }
}
} // namespace audio_agent
