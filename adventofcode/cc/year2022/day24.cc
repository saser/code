#include "adventofcode/cc/year2022/day24.h"

#include <cmath>
#include <queue>
#include <string>
#include <tuple>
#include <vector>

#include "absl/container/flat_hash_set.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"
#include "adventofcode/cc/geometry/pos.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day24 {
namespace {
class Cave {
 public:
  Cave(absl::string_view input) {
    cave_ = absl::StrSplit(input, '\n', absl::SkipEmpty());
    width_ = cave_[0].size() - 2;  // Everything except left and right walls.
    height_ = cave_.size() - 2;    // Everything except top and bottom walls.
  }

  adventofcode::cc::geometry::Pos Entrance() const {
    return adventofcode::cc::geometry::Pos{.x = 1, .y = 0};
  }

  adventofcode::cc::geometry::Pos Exit() const {
    return adventofcode::cc::geometry::Pos{.x = int(width_),
                                           .y = int(height_) + 1};
  }

  bool Occupied(const adventofcode::cc::geometry::Pos& pos, size_t time) const {
    return cave_[pos.y][pos.x] == '#' ||  // Check for walls.
           OccupiedUp(pos, time) || OccupiedDown(pos, time) ||
           OccupiedLeft(pos, time) || OccupiedRight(pos, time);
  }

  size_t ShortestPath(const adventofcode::cc::geometry::Pos& src, size_t time,
                      const adventofcode::cc::geometry::Pos& dst) const {
    using state = std::tuple<adventofcode::cc::geometry::Pos,
                             size_t>;  // Position and time.
    // h is the heuristic function. It calculates the Manhattan distance to the
    // destination.
    auto h = [&dst](const adventofcode::cc::geometry::Pos& p) -> int64_t {
      return p.Distance(dst);
    };
    // less implements the ordering used by the priority queue. The queue uses a
    // less function to implement a priority queue where the _maximum_ element
    // is placed first. Therefore, to get the _minimum_ element first (which is
    // what we want here), we need to reverse the logic of the less function.
    auto less = [h](const state& s1, const state& s2) -> bool {
      const auto& [p1, t1] = s1;
      const auto& [p2, t2] = s2;

      return (t1 + h(p1)) > (t2 + h(p2));
    };
    absl::flat_hash_set<state> seen;
    std::priority_queue<state, std::vector<state>, decltype(less)> q(less);
    // Starting point: entrance of cave at time 0.
    q.push({src, time});
    while (!q.empty()) {
      state s = q.top();
      q.pop();
      if (seen.contains(s)) {
        continue;
      }
      seen.insert(s);
      const auto& [pos, t] = s;
      if (pos == dst) {
        return t - time;
      }
      for (const adventofcode::cc::geometry::Pos& d : {
               adventofcode::cc::geometry::Pos{.x = 0, .y = 0},   // Wait.
               adventofcode::cc::geometry::Pos{.x = 0, .y = -1},  // Up.
               adventofcode::cc::geometry::Pos{.x = 0, .y = +1},  // Down.
               adventofcode::cc::geometry::Pos{.x = -1, .y = 0},  // Left.
               adventofcode::cc::geometry::Pos{.x = +1, .y = 0},  // Right.
           }) {
        adventofcode::cc::geometry::Pos pos2 = pos + d;
        size_t t2 = t + 1;
        if (pos2.x < 0 || pos2.x >= width_ + 1 || pos2.y < 0 ||
            pos2.y >= height_ + 2) {
          // Out of bounds.
          continue;
        }
        if (Occupied(pos2, t2)) {
          // Cannot move to the new position.
          continue;
        }
        q.push({pos2, t2});
      }
    }
    return -1;
  }

 private:
  // The Occupied* methods return true if a blizzard is occupying the given
  // position at the given time. We use a little bit of math to make them O(1)
  // operations. Using '<' (left) blizzards as an example, the main idea is that
  // a position (x, y) being occupied at time T is equivalent to position (x+1,
  // y) being occupied at time T-1. This induction-style argument means that we
  // can calculate whether (x, y) is occupied at time T by checking whether
  // (x+T, y) is occupied at time 0 -- and this we can look up directly in the
  // input.
  //
  // We use modular arithmetic to account for the fact that blizzards loop
  // around the walls. The width_ and height_ variables help with this.
  //
  // We need to be careful with translating between coordinate systems. The
  // input positions consider (0,0) to be the top left of the input -- i.e. the
  // wall next to the entrance. For the modular arithmetic to work, we need to
  // consider (0,0) to be the top left of the cave, which is (1,1) in the input
  // positions. That's why there is code that adds and subtracts 1 in these
  // functions -- to translate between these coordinate systems.

  bool OccupiedUp(const adventofcode::cc::geometry::Pos& pos,
                  size_t time) const {
    //    y is occupied at T if y+1 is occupied at T-1.
    // => y is occupied at T if y+T is occupied at 0.
    // => y is occupied at T if (y+T) mod H is occupied at 0.
    size_t y = pos.y - 1;      // Coordinate system translation.
    y = (y + time) % height_;  // Modulo arithmetic.
    y += 1;                    // Coordinate system translation.
    return cave_[y][pos.x] == '^';
  }

  bool OccupiedDown(const adventofcode::cc::geometry::Pos& pos,
                    size_t time) const {
    //    y is occupied at T if y-1 is occupied at T-1.
    // => y is occupied at T if y-T is occupied at 0.
    // => y is occupied at T if (y-T) mod H is occupied at 0.
    // y-T can underflow since size_t is unsigned. However:
    // * if T' = T % H, then (y-T) mod H == (y-T') mod H.
    // * (y-T') can still underflow, but we can get around that by noting that
    //   (y-T') mod H == (y-T'+H) mod H.
    //
    // All in all, we calculate y as follows:
    size_t y = pos.y - 1;  // Coordinate system translation.
    time %= height_;       // Make sure that 0 <= time < height_.
    if (y < time) {
      y += height_;
    }
    y = (y - time) % height_;
    y += 1;  // Coordinate system translation.
    return cave_[y][pos.x] == 'v';
  }

  bool OccupiedLeft(const adventofcode::cc::geometry::Pos& pos,
                    size_t time) const {
    //    x is occupied at T if x+1 is occupied at T-1.
    // => x is occupied at T if x+T is occupied at 0.
    // => x is occupied at T if (x+T) mod W is occupied at 0.
    size_t x = pos.x - 1;     // Coordinate system translation.
    x = (x + time) % width_;  // Modulo arithmetic.
    x += 1;                   // Coordinate system translation.
    return cave_[pos.y][x] == '<';
  }

  bool OccupiedRight(const adventofcode::cc::geometry::Pos& pos,
                     size_t time) const {
    //    x is occupied at T if x-1 is occupied at T-1.
    // => x is occupied at T if x-T is occupied at 0.
    // => x is occupied at T if (x-T) mod W is occupied at 0.
    // x-T can underflow since size_t is unsigned. However:
    // * if T' = T % W, then (x-T) mod W == (x-T') mod W.
    // * (x-T') can still underflow, but we can get around that by noting that
    //   (x-T') mod W == (x-T'+W) mod W.
    //
    // All in all, we calculate x as follows:
    size_t x = pos.x - 1;  // Coordinate system translation.
    time %= width_;        // Make sure that 0 <= time < width_.
    if (x < time) {
      x += width_;
    }
    x = (x - time) % width_;
    x += 1;  // Coordinate system translation.
    return cave_[pos.y][x] == '>';
  }

  // The entire input, split into lines.
  std::vector<absl::string_view> cave_;
  // The width and height of the cave, _excluding_ the walls. In other words,
  // this cave has a width of 2 and height of 2:
  //     #.##
  //     #>v#
  //     #^<#
  //     ##.#
  size_t width_, height_;
};

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  Cave c(input);
  adventofcode::cc::geometry::Pos entrance = c.Entrance();
  adventofcode::cc::geometry::Pos exit = c.Exit();
  size_t time = 0;
  time += c.ShortestPath(entrance, time, exit);
  if (part1) {
    return std::to_string(time);
  }
  time += c.ShortestPath(exit, time, entrance);
  time += c.ShortestPath(entrance, time, exit);
  return std::to_string(time);
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  Cave c(input);
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}
}  // namespace day24
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
