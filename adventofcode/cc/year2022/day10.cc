#include "adventofcode/cc/year2022/day10.h"

#include <sstream>
#include <string>
#include <vector>

#include "absl/log/check.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day10 {
namespace {

class CPU {
 public:
  CPU() = delete;
  CPU(std::vector<absl::string_view> instructions)
      : instructions_(instructions),
        pc_(0),
        x_(1),
        elapsed_(0),
        breakpoint_(20),
        sum_(0),
        screen_{} {}

  // Execute executes the current instruction and returns true if there are
  // instructions remaining in the program.
  bool Execute() {
    absl::string_view instruction = instructions_[pc_];
    if (instruction == "noop") {
      cycle();
    } else {
      int arg;
      CHECK(absl::SimpleAtoi(instruction.substr(5), &arg)) << instruction;
      // An "addx" instruction executes for two cycles, and it's after the
      // second cycle has elapsed that the value can be said to take effect.
      cycle();
      cycle();
      x_ += arg;
    }
    pc_++;
    return pc_ < instructions_.size();
  }

  int Sum() const { return sum_; }

  std::string Print() const {
    std::stringstream s;
    for (size_t row = 0; row < kScreenHeight; row++) {
      for (size_t col = 0; col < kScreenWidth; col++) {
        s << (screen_[row * kScreenWidth + col] ? '#' : '.');
      }
      if (row < kScreenHeight - 1) {
        s << '\n';
      }
    }
    return s.str();
  }

 private:
  void cycle() {
    // For part 2: screen drawing happens "during" the cycle, meaning its
    // effects are seen after the cycle completes -- and therefore uses data
    // from before elapsed_ has been increased.
    size_t col = elapsed_ % kScreenWidth;
    screen_[elapsed_] = col == x_ - 1 || col == x_ || col == x_ + 1;
    elapsed_++;
    // For part 1.
    if (elapsed_ == breakpoint_) {
      sum_ += x_ * elapsed_;
      breakpoint_ += 40;
    }
  }

  std::vector<absl::string_view> instructions_;  // The program.
  size_t pc_;                                    // Index of next instruction.

  size_t x_;           // The X register.
  size_t elapsed_;     // Number of completed cycles.
  size_t breakpoint_;  // After which cycle signal strength should be read next.
  size_t sum_;         // Sum of signal strengths.

  // For part 2.
  inline static constexpr size_t kScreenHeight = 6;
  inline static constexpr size_t kScreenWidth = 40;
  bool screen_[kScreenWidth * kScreenHeight];
};

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  std::vector<absl::string_view> instructions =
      absl::StrSplit(input, '\n', absl::SkipEmpty());
  CPU cpu(instructions);
  while (cpu.Execute()) {
    // Do nothing; the .Execute() call is what has side effects.
  }
  if (part1) {
    return std::to_string(cpu.Sum());
  } else {
    return cpu.Print();
  }
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}

}  // namespace day10
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
