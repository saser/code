#include "adventofcode/cc/year2022/day09.h"

#include <cmath>
#include <string>
#include <utility>

#include "absl/container/flat_hash_set.h"
#include "absl/hash/hash.h"
#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_format.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day09 {
namespace {
struct Position {
  int x;
  int y;

  std::string String() const { return absl::StrFormat("(%d,%d)", x, y); }

  // clangd complains that this is unused, but it is actually required by the
  // Abseil hashing stuff below, hence why the linter is suppressed here.
  // NOLINTNEXTLINE
  friend inline bool operator==(const Position& lhs, const Position& rhs) {
    return lhs.x == rhs.x && lhs.y == rhs.y;
  }

  template <typename H>
  friend H AbslHashValue(H h, const Position& p) {
    return H::combine(std::move(h), p.x, p.y);
  }
};

// Returns -1 if val < 0, 0 if val == 0, and +1 if val > 0.
// Taken from https://stackoverflow.com/a/4609795.
int sgn(int val) { return (0 < val) - (val < 0); }

class Rope {
 public:
  Rope() = delete;
  Rope(size_t n) : knots_(std::vector<Position>(n)) {}

  enum class Direction {
    kUp,
    kDown,
    kLeft,
    kRight,
  };

  void Step(Direction dir) {
    Position& head = knots_[0];
    switch (dir) {
      case Direction::kUp:
        head.y++;
        break;
      case Direction::kDown:
        head.y--;
        break;
      case Direction::kLeft:
        head.x--;
        break;
      case Direction::kRight:
        head.x++;
        break;
    }
    for (size_t i = 1; i < knots_.size(); i++) {
      Position& knot = knots_[i];
      Position& previous = knots_[i - 1];
      int dx = previous.x - knot.x;
      int dy = previous.y - knot.y;
      if (std::abs(dx) > 1 || std::abs(dy) > 1) {
        knot.x += sgn(dx);
        knot.y += sgn(dy);
      }
    }
  }

  Position LastKnotPosition() const { return knots_.back(); }

 private:
  std::vector<Position> knots_;
};

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  Rope r(part1 ? 2 : 10);
  absl::flat_hash_set<Position> tail_positions;
  for (absl::string_view line :
       absl::StrSplit(input, '\n', absl::SkipEmpty())) {
    Rope::Direction dir;
    switch (line[0]) {
      case 'U':
        dir = Rope::Direction::kUp;
        break;
      case 'D':
        dir = Rope::Direction::kDown;
        break;
      case 'L':
        dir = Rope::Direction::kLeft;
        break;
      case 'R':
        dir = Rope::Direction::kRight;
        break;
    }
    int n;
    if (!absl::SimpleAtoi(line.substr(2), &n)) {
      return absl::InvalidArgumentError(
          absl::StrFormat("invalid line: %s", line));
    }
    for (int i = 0; i < n; i++) {
      r.Step(dir);
      tail_positions.insert(r.LastKnotPosition());
    }
  }
  return std::to_string(tail_positions.size());
}

}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}

}  // namespace day09
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
