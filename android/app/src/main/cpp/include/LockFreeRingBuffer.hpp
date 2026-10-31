#pragma once

#include <atomic>
#include <vector>
#include <cstring>
#include <algorithm>
#include <cstddef>

/**
 * Single-Producer Single-Consumer (SPSC) Lock-Free RingBuffer.
 * Designed for real-time audio threads (e.g. Oboe FastMixer / FastCapture).
 * Guaranteed zero-lock, zero heap allocation during write/read.
 * 64-byte alignment prevents false sharing across CPU cores.
 */
template <typename T>
class LockFreeSPSCRingBuffer {
public:
    explicit LockFreeSPSCRingBuffer(size_t capacity)
        : capacity_(capacity + 1), buffer_(capacity + 1), data_ptr_(buffer_.data()) {}

    ~LockFreeSPSCRingBuffer() = default;

    // Non-copyable, movable
    LockFreeSPSCRingBuffer(const LockFreeSPSCRingBuffer&) = delete;
    LockFreeSPSCRingBuffer& operator=(const LockFreeSPSCRingBuffer&) = delete;
    LockFreeSPSCRingBuffer(LockFreeSPSCRingBuffer&& other) noexcept
        : capacity_(other.capacity_), buffer_(std::move(other.buffer_)),
          data_ptr_(buffer_.data()),
          head_(other.head_.load(std::memory_order_relaxed)),
          tail_(other.tail_.load(std::memory_order_relaxed)) {}
    LockFreeSPSCRingBuffer& operator=(LockFreeSPSCRingBuffer&&) noexcept = delete;
    /**
     * Producer writes up to `count` elements into the ring buffer.
     * Returns the actual number of elements written.
     */
    size_t write(const T* __restrict__ data, size_t count) {
        if (!data || count == 0) return 0;

        const size_t head = head_.load(std::memory_order_relaxed);
        const size_t tail = tail_.load(std::memory_order_acquire);
        const size_t free_space = (tail > head) ? (tail - head - 1) : (capacity_ + tail - head - 1);
        const size_t to_write = std::min(count, free_space);
        if (__builtin_expect(to_write == 0, 0)) return 0;

        const size_t first_chunk = std::min(to_write, capacity_ - head);
        std::memcpy(data_ptr_ + head, data, first_chunk * sizeof(T));
        if (__builtin_expect(to_write > first_chunk, 0)) {
            std::memcpy(data_ptr_, data + first_chunk, (to_write - first_chunk) * sizeof(T));
        }
        size_t next_head = head + to_write;
        if (next_head >= capacity_) next_head -= capacity_;
        head_.store(next_head, std::memory_order_release);
        return to_write;
    }

    /**
     * Consumer reads up to `count` elements from the ring buffer.
     * Returns the actual number of elements read.
     */
    size_t read(T* __restrict__ dest, size_t count) {
        if (!dest || count == 0) return 0;

        const size_t tail = tail_.load(std::memory_order_relaxed);
        const size_t head = head_.load(std::memory_order_acquire);
        const size_t available = (head >= tail) ? (head - tail) : (capacity_ + head - tail);
        const size_t to_read = std::min(count, available);
        if (__builtin_expect(to_read == 0, 0)) return 0;

        const size_t first_chunk = std::min(to_read, capacity_ - tail);
        std::memcpy(dest, data_ptr_ + tail, first_chunk * sizeof(T));
        if (__builtin_expect(to_read > first_chunk, 0)) {
            std::memcpy(dest + first_chunk, data_ptr_, (to_read - first_chunk) * sizeof(T));
        }
        size_t next_tail = tail + to_read;
        if (next_tail >= capacity_) next_tail -= capacity_;
        tail_.store(next_tail, std::memory_order_release);
        return to_read;
    }

    /**
     * Instantly purges the ring buffer. Safe to call on Barge-in.
     */
    void clear() {
        tail_.store(head_.load(std::memory_order_acquire), std::memory_order_release);
    }

    size_t available_read() const {
        const size_t tail = tail_.load(std::memory_order_relaxed);
        const size_t head = head_.load(std::memory_order_acquire);
        return (head >= tail) ? (head - tail) : (capacity_ + head - tail);
    }

    size_t available_write() const {
        const size_t head = head_.load(std::memory_order_relaxed);
        const size_t tail = tail_.load(std::memory_order_acquire);
        return (tail > head) ? (tail - head - 1) : (capacity_ + tail - head - 1);
    }

    size_t capacity() const {
        return capacity_ - 1;
    }

    bool empty() const {
        return available_read() == 0;
    }

private:
    const size_t capacity_;
    std::vector<T> buffer_;
    T* data_ptr_{nullptr};
    alignas(64) std::atomic<size_t> head_{0}; // 64-byte cache line alignment prevents false sharing
    alignas(64) std::atomic<size_t> tail_{0};
};
