#include "adventofcode/cc/year2022/day17.h"

#include <cstdint>
#include <sstream>
#include <string>

#include "absl/container/flat_hash_map.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/string_view.h"
#include "adventofcode/cc/trim.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day17 {
namespace {

// Idea: use bitmasks to represent everything.
//
// Shapes are represented as 32-bit integers, 8 bits per row. Example
// using the cross shape:
//
//                bit numbers
//     00000000 | 31 -> 24
//     00000010 | 23 -> 16
//     00000111 | 15 ->  8
//     00000010 |  7 ->  0
//
// The tower is represented as a vector of 8-bit integers (I don't dare say this
// is bytes, because it can differ based on platform). Element 0 in the vector
// is the first row, element 1 is the second row, etc. Example of the tower
// after placing a cross shape (1) and then a pole shape next to it (2). While
// they are marked with 1 and 2, they are both set to 1 in the bit
// representation. The imaginary walls of the tower are represented with #,
// although they are not present in the bit representation (see below).
//
//     76543210  | bit number
//
//     #0000002# | element 3
//     #0000102# | element 2
//     #0001112# | element 1
//     #0000102# | element 0
//
// Detecting when a shape is at the edge is done with two special bitmasks. The
// 1's in the bitmasks represent the leftmost and rightmost bits that can be
// present for a shape. The special bitmasks looks like this, again with #
// representing the walls of the tower but not present in the bitmask:
//
//     right       bit numbers
//     #0000001# | 31 -> 24
//     #0000001# | 23 -> 16
//     #0000001# | 15 ->  8
//     #0000001# |  7 ->  0
//     76543210
//
//     left        bit numbers
//     #1000000# | 31 -> 24
//     #1000000# | 23 -> 16
//     #1000000# | 15 ->  8
//     #1000000# |  7 ->  0
//     76543210
//
// Using the cross shape example from above, here's how to detect that it is at
// the right edge and cannot move further right:
//
//     cross      edges                 bit numbers
//     00000000   01000001   00000000 | 31 -> 24
//     00000010   01000001   00000000 | 23 -> 16
//     00000111   01000001   00000001 | 15 ->  8
//     00000010 & 01000001 = 00000000 |  7 ->  0
//
// The result is non-zero, which means the shape is at the edge.
//
// Bringing all of this together, here is how to implement the movements in
// Tetris:
//
//   * Moving a shape left: shape <<= 1
//   * Moving a shape right: shape >>= 1
//   * Detecting edges: (shape & edgemask) != 0
//   * Detecting rocks at rest: rockmask := up to 4 tower integers put together
//     to form a 32-bit integer; then (shape & rockmask) != 0.
//   * Moving downwards: move the "window" of tower integers from [n, n-4] to
//     [n-1, n-5].

// The starting bit representations of the shapes are created so that when they
// spawn into the tower, they spawn so that their left edge is two units away
// from the left wall and their bottom edge is as low as possible.

static inline constexpr uint32_t kBar = (0b00000000 << 24) |  // 31 -> 24
                                        (0b00000000 << 16) |  // 23 -> 16
                                        (0b00000000 << 8) |   // 15 ->  8
                                        (0b00011110 << 0);    //  7 ->  0

static inline constexpr uint32_t kCross = (0b00000000 << 24) |  // 31 -> 24
                                          (0b00001000 << 16) |  // 23 -> 16
                                          (0b00011100 << 8) |   // 15 ->  8
                                          (0b00001000 << 0);    //  7 ->  0

static inline constexpr uint32_t kCorner = (0b00000000 << 24) |  // 31 -> 24
                                           (0b00000100 << 16) |  // 23 -> 16
                                           (0b00000100 << 8) |   // 15 ->  8
                                           (0b00011100 << 0);    //  7 ->  0

static inline constexpr uint32_t kPole = (0b00010000 << 24) |  // 31 -> 24
                                         (0b00010000 << 16) |  // 23 -> 16
                                         (0b00010000 << 8) |   // 15 ->  8
                                         (0b00010000 << 0);    //  7 ->  0

static inline constexpr uint32_t kBox = (0b00000000 << 24) |  // 31 -> 24
                                        (0b00000000 << 16) |  // 23 -> 16
                                        (0b00011000 << 8) |   // 15 ->  8
                                        (0b00011000 << 0);    //  7 ->  0

// See the description above for what this represents and how it's used.
static inline constexpr uint32_t kRightEdge = (0b00000001 << 24) |  // 31 -> 24
                                              (0b00000001 << 16) |  // 23 -> 16
                                              (0b00000001 << 8) |   // 15 ->  8
                                              (0b00000001 << 0);    //  7 ->  0

static inline constexpr uint32_t kLeftEdge = (0b01000000 << 24) |  // 31 -> 24
                                             (0b01000000 << 16) |  // 23 -> 16
                                             (0b01000000 << 8) |   // 15 ->  8
                                             (0b01000000 << 0);    //  7 ->  0

static inline constexpr size_t kShapeCount = 5;
static inline constexpr size_t kTowerWidth = 7;

static inline constexpr uint32_t kShapeRotation[kShapeCount] = {
    kBar, kCross, kCorner, kPole, kBox,
};

uint8_t Subshape(uint32_t rock, size_t offset) {
  assert(offset >= 0 && offset < 4);
  return rock >> (8 * offset);
}

// I'm keeping this function around because it's useful for debugging, even
// though I'm not using it.
// std::string RockString(uint32_t rock) {
//   std::stringstream buf;
//   for (size_t i = 4; i >= 1; i--) {
//     size_t offset = i - 1;
//     buf << '|';
//     uint8_t subshape = Subshape(rock, offset);
//     for (uint8_t j = 7; j >= 1; j--) {
//       uint8_t bitnum = j - 1;
//       bool is_set = (subshape & (1 << bitnum)) != 0;
//       buf << (is_set ? '#' : '.');
//     }
//     buf << '|';
//     buf << std::endl;
//   }
//   return buf.str();
// }

class Tetris {
 public:
  Tetris() = delete;
  Tetris(absl::string_view input)
      : input_(input), next_shape_(0), next_jet_push_(0), tower_() {}

  size_t TowerHeight() const { return tower_.size(); }

  void DropRock() {
    uint32_t rock = kShapeRotation[next_shape_];
    next_shape_ = (next_shape_ + 1) % kShapeCount;
    size_t bottom = tower_.size() + 3;
    while (true) {
      bool push_left = input_[next_jet_push_] == '<';

      uint32_t edge_mask = push_left ? kLeftEdge : kRightEdge;
      uint32_t rock_mask = RockMask(bottom);
      // shifted represents the new representation of the rock, if it is able
      // to move to the left or right.
      uint32_t shifted = push_left ? (rock << 1) : (rock >> 1);
      // This is a bit gnarly, so:
      //
      // * (shape & edge_mask) == 0: true if the _current_ rock is away from
      //   the relevant edge.
      // * (shifted & rock_mask) == 0: true if the _shifted_ rock is away from
      //   any rocks at rest.
      //
      // If both of these conditions are true, then the rock can move.
      if ((rock & edge_mask) == 0 && (shifted & rock_mask) == 0) {
        rock = shifted;
      }
      next_jet_push_ = (next_jet_push_ + 1) % input_.size();

      // Next, figure out if the shape would interfere with any rocks when moved
      // one step down. If so, then the rock in its current position will come
      // to rest. As a special case, if the rock is at the very bottom of the
      // tower (bottom == 0), then it will also come to rest.
      if (bottom == 0 || (rock & RockMask(bottom - 1)) != 0) {
        PutToRest(rock, bottom);
        break;
      }
      bottom--;
    }
  }

  std::string DebugString() const {
    std::stringstream buf;
    for (auto it = tower_.crbegin(); it != tower_.crend(); it++) {
      buf << '|';
      for (int bitnum = 6; bitnum >= 0; bitnum--) {
        bool is_set = (*it & (1 << bitnum)) != 0;
        buf << (is_set ? '#' : '.');
      }
      buf << '|';
      buf << std::endl;
    }
    return buf.str();
  }

  struct StateKey {
    size_t next_shape;
    size_t next_jet_push;
    uint64_t tower_top8;
    size_t height_profile[kTowerWidth];

    // NOLINTNEXTLINE
    friend bool operator==(const StateKey& lhs, const StateKey& rhs) {
      return lhs.next_shape == rhs.next_shape &&
             lhs.next_jet_push == rhs.next_jet_push &&
             lhs.tower_top8 == rhs.tower_top8;
    }

    template <typename H>
    friend H AbslHashValue(H h, const StateKey& k) {
      h = H::combine(std::move(h), k.next_shape, k.next_jet_push);
      h = H::combine_contiguous(std::move(h), k.height_profile, kTowerWidth);
      return h;
    }
  };

  StateKey State() const {
    StateKey k;
    k.next_shape = next_shape_;
    k.next_jet_push = next_jet_push_;
    uint8_t seen = 0b00000000;
    constexpr uint8_t done = 0b01111111;
    size_t height = tower_.size();
    for (auto it = tower_.crbegin(); it != tower_.crend(); it++) {
      for (size_t bitnum = 0; bitnum < kTowerWidth; bitnum++) {
        uint8_t mask = 1 << bitnum;
        if ((seen & mask) != 0) {
          // We have already recorded a height for this column, so skip it.
          continue;
        }
        seen |= mask;
        k.height_profile[bitnum] = height;
      }
      if (seen == done) {
        break;
      }
      height--;
    }
    return k;
  }

 private:
  uint32_t RockMask(size_t bottom) const {
    uint32_t mask = 0;
    for (size_t row = bottom; row < tower_.size() && row < bottom + 4; row++) {
      mask |= uint32_t(tower_[row]) << (8 * (row - bottom));
    }
    return mask;
  }

  void PutToRest(uint32_t shape, size_t bottom) {
    for (size_t offset = 0; offset < 4; offset++) {
      uint8_t subshape = Subshape(shape, offset);
      if (subshape == 0) {
        break;
      }
      size_t row = bottom + offset;
      if (row >= tower_.size()) {
        tower_.resize(row + 1);
      }
      tower_[row] = tower_[row] | subshape;
    }
  }

  absl::string_view input_;
  size_t next_shape_;
  size_t next_jet_push_;
  std::vector<uint8_t> tower_;
};

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  Tetris t(adventofcode::cc::trim::TrimSpace(input));

  std::vector<size_t> seen_heights;
  seen_heights.push_back(t.TowerHeight());
  absl::flat_hash_map<Tetris::StateKey, size_t> seen_states;
  seen_states[t.State()] = 0;

  size_t kRocks = part1 ? 2022 : 1'000'000'000'000;
  size_t loop_start = 0;
  size_t loop_end = 0;
  for (size_t i = 1; i <= kRocks; i++) {
    t.DropRock();
    seen_heights.push_back(t.TowerHeight());
    Tetris::StateKey state = t.State();
    auto it = seen_states.find(state);
    if (it != seen_states.end()) {
      loop_start = it->second;
      loop_end = i;
      break;
    }
    seen_states[state] = i;
  }
  if (loop_start == 0 && loop_end == 0) {
    return std::to_string(seen_heights.back());
  }
  size_t loop_length = loop_end - loop_start;
  size_t total_steps_in_loop = kRocks - loop_start;
  size_t loop_iterations = total_steps_in_loop / loop_length;
  size_t loop_rest = total_steps_in_loop % loop_length;
  size_t loop_diff = seen_heights[loop_end] - seen_heights[loop_start];
  size_t total_height =
      loop_iterations * loop_diff + seen_heights[loop_start + loop_rest];
  return std::to_string(total_height);
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}
}  // namespace day17
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
