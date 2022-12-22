#include "adventofcode/cc/year2022/day23.h"

#include <algorithm>
#include <optional>
#include <sstream>
#include <string>
#include <tuple>
#include <vector>

#include "absl/container/flat_hash_map.h"
#include "absl/container/flat_hash_set.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/str_format.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day23 {
namespace {

struct Pos {
  int x;  // Increases from left to right.
  int y;  // Increases from top to bottom.

  std::string String() const { return absl::StrFormat("(%d,%d)", x, y); }

  // clangd complains that this is unused, but it is actually required by the
  // Abseil hashing stuff below, hence why the linter is suppressed here.
  // NOLINTNEXTLINE
  friend inline bool operator==(const Pos& lhs, const Pos& rhs) {
    return lhs.x == rhs.x && lhs.y == rhs.y;
  }

  template <typename H>
  friend H AbslHashValue(H h, const Pos& p) {
    return H::combine(std::move(h), p.x, p.y);
  }
};

constexpr inline Pos NW{.x = -1, .y = -1};
constexpr inline Pos N{.x = 0, .y = -1};
constexpr inline Pos NE{.x = +1, .y = -1};
constexpr inline Pos E{.x = +1, .y = 0};
constexpr inline Pos W{.x = -1, .y = 0};
constexpr inline Pos SW{.x = -1, .y = +1};
constexpr inline Pos S{.x = 0, .y = +1};
constexpr inline Pos SE{.x = +1, .y = +1};

class Elves {
 public:
  static Elves Parse(absl::string_view input) {
    Elves e;
    int y = 0;
    for (absl::string_view line :
         absl::StrSplit(input, '\n', absl::SkipEmpty())) {
      for (size_t x = 0; x < line.size(); x++) {
        if (line[x] == '#') {
          e.occupied_.insert(Pos{.x = int(x), .y = y});
        }
      }
      y++;
    }
    return e;
  }

  std::string String() const {
    std::stringstream s;
    const auto [min_x, max_x, min_y, max_y] = Bounds();
    for (int y = min_y; y <= max_y; y++) {
      for (int x = min_x; x <= max_x; x++) {
        s << (occupied_.contains(Pos{.x = x, .y = y}) ? '#' : '.');
      }
      if (y < max_y) {
        s << std::endl;
      }
    }
    return s.str();
  }

  // Perform one round and return whether any Elf moved.
  bool Round() {
    // Collect a list of moves. The mapping is from destination to a list of
    // sources. The reason is that several Elves can propose to move to the same
    // destination, and in that case none of them should move.
    absl::flat_hash_map<Pos, std::vector<Pos>> moves;
    for (const Pos& src : occupied_) {
      std::vector<Pos> proposals;
      for (char dir : directions_) {
        std::optional<Pos> dst;
        switch (dir) {
          case 'N':
            dst = ProposeNorth(src);
            break;
          case 'S':
            dst = ProposeSouth(src);
            break;
          case 'W':
            dst = ProposeWest(src);
            break;
          case 'E':
            dst = ProposeEast(src);
            break;
        }
        if (dst.has_value()) {
          proposals.push_back(dst.value());
        }
      }
      // If _all_ directions are possible, it means the Elf is surrounded by
      // empty space. If _no_ directions are possible, it means the Elf cannot
      // move anywhere. In both of these cases, the Elf should do nothing.
      if (size_t n = proposals.size(); n == 0 || n == 4) {
        continue;
      }
      // Otherwise, the Elf is able to move. The list of proposed new positions
      // is already ordered according to preference, so we take the first one.
      moves[proposals.front()].push_back(src);
    }
    // Once all moves have been collected, perform them all at once.
    bool any_moved = false;
    for (const auto& [dst, srcs] : moves) {
      // If more than one Elf has proposed this move, skip it.
      if (srcs.size() > 1) {
        continue;
      }
      occupied_.erase(srcs.front());
      occupied_.insert(dst);
      any_moved = true;
    }
    // Rotate the directions by removing the first element and putting it at the
    // back.
    char dir = directions_.front();
    directions_.erase(directions_.begin());
    directions_.push_back(dir);
    return any_moved;
  }

  int EmptyGround() const {
    const auto [min_x, max_x, min_y, max_y] = Bounds();
    int width = max_x - min_x + 1;
    int height = max_y - min_y + 1;
    return width * height - occupied_.size();
  }

 private:
  Elves() : occupied_(), directions_({'N', 'S', 'W', 'E'}){};

  std::tuple<int, int, int, int> Bounds() const {
    auto cmp_x = [](const Pos& p1, const Pos& p2) -> bool {
      return p1.x < p2.x;
    };
    auto cmp_y = [](const Pos& p1, const Pos& p2) -> bool {
      return p1.y < p2.y;
    };
    int min_x =
        std::min_element(occupied_.cbegin(), occupied_.cend(), cmp_x)->x;
    int max_x =
        std::max_element(occupied_.cbegin(), occupied_.cend(), cmp_x)->x;
    int min_y =
        std::min_element(occupied_.cbegin(), occupied_.cend(), cmp_y)->y;
    int max_y =
        std::max_element(occupied_.cbegin(), occupied_.cend(), cmp_y)->y;
    return {min_x, max_x, min_y, max_y};
  }

  std::optional<Pos> ProposeNorth(const Pos& p) const {
    for (const Pos& d : {NW, N, NE}) {
      if (occupied_.contains(Pos{.x = p.x + d.x, .y = p.y + d.y})) {
        return std::nullopt;
      }
    }
    return Pos{.x = p.x + N.x, .y = p.y + N.y};
  }

  std::optional<Pos> ProposeSouth(const Pos& p) const {
    for (const Pos& d : {SW, S, SE}) {
      if (occupied_.contains(Pos{.x = p.x + d.x, .y = p.y + d.y})) {
        return std::nullopt;
      }
    }
    return Pos{.x = p.x + S.x, .y = p.y + S.y};
  }

  std::optional<Pos> ProposeWest(const Pos& p) const {
    for (const Pos& d : {W, NW, SW}) {
      if (occupied_.contains(Pos{.x = p.x + d.x, .y = p.y + d.y})) {
        return std::nullopt;
      }
    }
    return Pos{.x = p.x + W.x, .y = p.y + W.y};
  }

  std::optional<Pos> ProposeEast(const Pos& p) const {
    for (const Pos& d : {E, NE, SE}) {
      if (occupied_.contains(Pos{.x = p.x + d.x, .y = p.y + d.y})) {
        return std::nullopt;
      }
    }
    return Pos{.x = p.x + E.x, .y = p.y + E.y};
  }

  absl::flat_hash_set<Pos> occupied_;
  std::vector<char> directions_;
};

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  Elves e = Elves::Parse(input);
  if (part1) {
    for (int i = 0; i < 10; i++) {
      e.Round();
    }
    return std::to_string(e.EmptyGround());
  }
  int rounds = 1;
  while (e.Round()) {
    rounds++;
  }
  return std::to_string(rounds);
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}
}  // namespace day23
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
