#include "adventofcode/cc/year2022/day14.h"

#include <algorithm>
#include <cstdlib>
#include <optional>
#include <sstream>
#include <string>
#include <vector>

#include "absl/container/btree_map.h"
#include "absl/container/btree_set.h"
#include "absl/container/flat_hash_set.h"
#include "absl/log/check.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_format.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day14 {
namespace {

// Returns -1 if val < 0, 0 if val == 0, and +1 if val > 0.
// Taken from https://stackoverflow.com/a/4609795.
int sgn(int val) { return (0 < val) - (val < 0); }

struct Pos {
  int x;
  int y;

  std::string String() const { return absl::StrFormat("%d,%d", x, y); }

  // NOLINTNEXTLINE
  inline friend bool operator==(const Pos& lhs, const Pos& rhs) {
    return lhs.x == rhs.x && lhs.y == rhs.y;
  }

  template <typename H>
  friend H AbslHashValue(H h, const Pos& p) {
    return H::combine(std::move(h), p.x, p.y);
  }
};

struct Span {
  Pos from;
  Pos to;

  std::string String() const {
    return absl::StrFormat("%s -> %s", from.String(), to.String());
  }

  std::vector<Pos> Positions() const {
    std::vector<Pos> positions;
    positions.push_back(from);
    // Only one of the two loops below will execute, because only one of dx and
    // dy will be non-zero.
    int dx = to.x - from.x;
    int dy = to.y - from.y;
    for (int i = 1; i <= std::abs(dx); i++) {
      Pos p;
      p.x = from.x + i * sgn(dx);
      p.y = from.y;
      positions.push_back(p);
    }
    for (int i = 1; i <= std::abs(dy); i++) {
      Pos p;
      p.x = from.x;
      p.y = from.y + i * sgn(dy);
      positions.push_back(p);
    }
    return positions;
  }
};

std::vector<Span> ParseSpans(absl::string_view s) {
  std::vector<Pos> positions;
  for (absl::string_view pair : absl::StrSplit(s, " -> ")) {
    std::pair<absl::string_view, absl::string_view> xy =
        absl::StrSplit(pair, ',');
    Pos p;
    CHECK(absl::SimpleAtoi(xy.first, &p.x));
    CHECK(absl::SimpleAtoi(xy.second, &p.y));
    positions.push_back(p);
  }
  std::vector<Span> spans;
  for (size_t i = 0; i < positions.size() - 1; i++) {
    Span s;
    s.from = positions[i];
    s.to = positions[i + 1];
    spans.push_back(s);
  }
  return spans;
}

class Cave {
 public:
  void AddRocks(const Span& s) {
    for (const Pos& rock : s.Positions()) {
      rocks_.insert(rock);
      lowest_rock_y_ = std::max(lowest_rock_y_, rock.y);
    }
  }

  // DropSand drops a unit of sand, starting at 500,0. If the sand comes to
  // rest, DropSand returns the position it came to rest in.
  std::optional<Pos> DropSand() {
    Pos sand;
    sand.x = 500;
    sand.y = 0;
    while (true) {
      if (WillFallForever(sand)) {
        return std::nullopt;
      }
      Pos next;
      // 1. Check immediately below.
      next.x = sand.x;
      next.y = sand.y + 1;
      if (!IsBlocked(next)) {
        sand = next;
        continue;
      }
      // 2. Check down and to the left.
      next.x = sand.x - 1;
      next.y = sand.y + 1;
      if (!IsBlocked(next)) {
        sand = next;
        continue;
      }
      // 3. Check down and to the right.
      next.x = sand.x + 1;
      next.y = sand.y + 1;
      if (!IsBlocked(next)) {
        sand = next;
        continue;
      }
      // The unit of sand has come to rest.
      sand_.insert(sand);
      return sand;
    }
  }

  std::string String() const {
    std::stringstream buf;
    int min_x, min_y, max_x, max_y;
    {
      Pos p = *rocks_.begin();
      min_x = p.x;
      max_x = p.x;
      min_y = p.y;
      max_y = p.y;
    }
    for (const Pos& rock : rocks_) {
      min_x = std::min(min_x, rock.x);
      max_x = std::max(max_x, rock.x);
      min_y = std::min(min_y, rock.y);
      max_y = std::max(max_y, rock.y);
    }
    for (const Pos& sand : sand_) {
      min_x = std::min(min_x, sand.x);
      max_x = std::max(max_x, sand.x);
      min_y = std::min(min_y, sand.y);
      max_y = std::max(max_y, sand.y);
    }
    // We build the string line by line, which means that we iterate over y in
    // the outer loop (each y value is a line) and x in the inner loop (each x
    // value is a column in that line).rock
    for (int y = min_y; y <= max_y; y++) {
      if (y > min_y) {
        buf << std::endl;
      }
      for (int x = min_x; x <= max_x; x++) {
        Pos p;
        p.x = x;
        p.y = y;
        char c;
        if (rocks_.contains(p)) {
          c = '#';
        } else if (sand_.contains(p)) {
          c = 'o';
        } else {
          c = '.';
        }
        buf << c;
      }
    }
    return buf.str();
  }

 private:
  // WillFallForever returns true if there is nothing that would stop the fall
  // of a sand at position p. It assumes that p is a position that is neither a
  // rock nor a unit of sand at rest.
  bool WillFallForever(const Pos& p) const { return p.y >= lowest_rock_y_; }
  // IsBlocked returns whether p is occupied by anything, i.e., a rock or a unit
  // of sand at rest.
  bool IsBlocked(const Pos& p) const {
    return rocks_.contains(p) || sand_.contains(p);
  }

  absl::flat_hash_set<Pos> rocks_;
  int lowest_rock_y_ = -1;
  absl::flat_hash_set<Pos> sand_;
};

}  // namespace

// For part 1, we implement the logic quite literally.
// For part 2, we use a few observations to make a much faster solution. I
// haven't found a way to apply the algorithm there to part 1 yet, so for now
// part 1 runs much slower.

absl::StatusOr<std::string> Part1(absl::string_view input) {
  Cave c;
  for (absl::string_view line :
       absl::StrSplit(input, '\n', absl::SkipEmpty())) {
    for (const Span& s : ParseSpans(line)) {
      c.AddRocks(s);
    }
  }
  int sand = 0;
  while (c.DropSand() != std::nullopt) {
    sand++;
  }
  return std::to_string(sand);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  // rocks maps from y coordinate to the x coordinates of
  // rocks on that line.
  absl::btree_map<int, absl::btree_set<int>> rocks;
  for (absl::string_view line :
       absl::StrSplit(input, '\n', absl::SkipEmpty())) {
    for (const Span& s : ParseSpans(line)) {
      for (const Pos& p : s.Positions()) {
        rocks[p.y].insert(p.x);
      }
    }
  }
  // unreachable maps from y coordinate to the x coordinates of unreachable
  // positions on that line. An unreachable position is one where a unit of sand
  // will never be, not even while falling. Not all unreachable positions are
  // rocks, but all rocks are unreachable positions.
  absl::btree_map<int, absl::btree_set<int>> unreachable = rocks;
  // We observe the following fact:
  // For a given y; if (x-1, y), (x, y), and (x+1, y) are unreachable, then (x,
  // y+1) is unreachable. The argument for this is simple: if neither of these
  // coordinates on the y line are reachable, then no unit of sand will ever go
  // down, down-and-left, or down-and-right into the position on the y+1 line.
  //
  // From this we can build the complete set of unreachable positions in a
  // top-to-bottom fashion. We stop when we have found all unreachable positions
  // above the infinite line; there is no need to go any deeper than that (and
  // it would even give the incorrect results later when we rely on the number
  // of unreachable positions).

  // We get the largest key (i.e., max y coordinate of any rock) from rocks and
  // then add 2 to find the y coordinate of the infinite line.
  int infinite_line_y = rocks.crbegin()->first + 2;
  // The condition here means that y will at most be infinite_line_y-2, which is
  // what we want becasue the loop body will (potentially) add unreachable
  // positions at y+1 = infinite_line_y-1, which is the last line we want to add
  // to (see above).
  for (int y = unreachable.begin()->first; y < infinite_line_y - 1; y++) {
    if (!unreachable.contains(y)) {
      continue;
    }
    std::vector<int> xs(unreachable[y].begin(), unreachable[y].end());
    if (xs.size() < 3) {
      continue;
    }
    for (size_t i = 1; i < xs.size() - 1; i++) {
      int x_left = xs[i - 1];
      int x_mid = xs[i];
      int x_right = xs[i + 1];
      if (x_left == x_mid - 1 && x_right == x_mid + 1) {
        unreachable[y + 1].insert(x_mid);
      }
    }
  }
  // Now that we know which positions are unreachable, we can easily calculate
  // how many positions will be occupied by units of sand if we start spawning
  // from (500,0).
  //
  // Assume that we had a super simple map consisting only of the "infinite
  // line" at the bottom. In this case, sand will start building a "cone", along
  // the following lines:
  //
  //    o
  // #######
  //
  //    o
  //   ooo
  // #######
  //
  //    o
  //   ooo
  //  ooooo
  // #######
  //
  // etc.
  //
  // We know from our rocks above where this infinite line would be. We can use
  // this to calculate the size of cone: each line will have as many units of
  // sand as the one above it, plus 2. We can see that pattern above: the first
  // line has 1, the second has 3, and so on.
  //
  // The sand spawns from line 0. The infinite line is at some y_line. The units
  // of sand will land on y_line-1 at most. This means that there will be y_line
  // lines of sand. (If y_line = 1, then there would be 1 line; see illustration
  // above).
  //
  // Since we have a sequence of 1 + 3 + 5 + ... we can use the arithmetic sum
  // formula:
  //     sum = n(a_1 + a_n) / 2
  //       n = the number of lines
  //         = y_line
  //     a_1 = 1
  //     a_n = 2n - 1
  //     sum = n(1 + (2n - 1)) / 2
  //         = n(2n) / 2
  //         = n^2
  //         = y_line^2
  // That is the size of the cone. Then we just subtract the number of
  // unreachable positions (they are guaranteed to all be inside the cone), and
  // we have our answer.
  int sum = infinite_line_y * infinite_line_y;
  for (const auto& [_, xs] : unreachable) {
    sum -= xs.size();
  }
  return std::to_string(sum);
}

}  // namespace day14
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
