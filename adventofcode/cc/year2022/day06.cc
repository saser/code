#include "adventofcode/cc/year2022/day06.h"

#include <sstream>
#include <string>
#include <vector>

#include "absl/status/statusor.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day06 {
namespace {

struct Buffer {
  Buffer() = delete;
  Buffer(size_t capacity) : capacity_(capacity) {}

  std::string DebugString() const {
    std::stringstream s;
    // Print contents of current buffer, and mark where the current buffer
    // starts with [].
    s << "buf_=";
    for (size_t i = 0; i < capacity_; i++) {
      char c = buf_[i];
      if (i == buf_start_) {
        s << '[' << c << ']';
      } else {
        s << ' ' << c << ' ';
      }
      s << ' ';
    }
    s << "size_=" << size_;
    s << " | ";

    // Print the character counts, if they're non-zero.
    s << "seen_=";
    for (size_t i = 0; i < 26; i++) {
      int n = seen_[i];
      if (n != 0) {
        s << char(i + 'a') << ':' << n << ' ';
      }
    }
    s << " | ";

    // Print the different_ counter.
    s << "different_=" << different_;
    return s.str();
  }

  void added(char c) {
    size_t idx = c - 'a';
    seen_[idx]++;
    if (seen_[idx] == 1) {
      different_++;
    }
  }

  void dropped(char c) {
    size_t idx = c - 'a';
    seen_[idx]--;
    if (seen_[idx] == 0) {
      different_--;
    }
  }

  void Push(char c) {
    if (size_ < capacity_) {
      // In this code path we know that we have spare capacity. Because callers
      // can only push characters, never pop them, we only ever end up in this
      // code path before we have first reached capacity. As such, we know that
      // buf_start_ = 0 and size_ < buf_.size().
      buf_[size_] = c;
      size_++;
      added(c);
      return;
    }
    // When we advance the buffer start, the old position becomes the new buffer
    // end. This happens to both be (a) where the dropped character is and (b)
    // where the new character should be placed. Example:
    //
    //     encodes: "etle"
    //     buf_ = 'l' 'e' 'e' 't'
    //                     ^ buf_start
    //
    // Setting buf_end := buf_start_ and then advancing buf_start_ gives us :
    //
    //     encodes: "tlee"
    //     buf_ = 'l' 'e' 'e' 't'
    //                     ^   ^ buf_start_
    //                     | buf_end
    //
    // Finally, we push the new character -- say, 'g' -- by setting
    // buf_[buf_end] = 'g'.
    //
    //     encodes: "tleg"
    //     buf_ = 'l' 'e' 'g' 't'
    //                     ^   ^ buf_start_
    //                     | buf_end
    size_t buf_end = buf_start_;
    buf_start_++;
    if (buf_start_ == size_) {
      buf_start_ = 0;
    }
    // Mark the oldest character as dropped, then replace it with the new
    // character and mark the new one as added.
    dropped(buf_[buf_end]);
    buf_[buf_end] = c;
    added(c);
  }

  int Different() const { return different_; }

 private:
  char buf_[14] =
      {};  // The raw buffer. 14 is the max capacity we will ever need. The
           // logical capacity is determined by the capacity_ variable.
  size_t buf_start_ = 0;  // Index into buf_ where the buffer logically starts.
  size_t capacity_;       // # of elements the buffer can contain. This is the
                          // logical capacity.
  size_t size_ = 0;       // # of elements the buffer currently contains.

  int seen_[26] = {};  // Character -> # of occurrences in the buffer.
  int different_ = 0;  // # of different characters the buffer contains.
};

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  // Crop the newline, if there is one.
  if (input.back() == '\n') {
    input = input.substr(0, input.length() - 1);
  }
  size_t capacity = part1 ? 4 : 14;
  Buffer buf(capacity);
  for (size_t i = 0; i < input.length(); i++) {
    const char& c = input[i];
    buf.Push(c);
    if (i >= capacity && buf.Different() == capacity) {
      return std::to_string(i + 1);
    }
  }
  return absl::InternalError("no answer found");
}

}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}

}  // namespace day06
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
