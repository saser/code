#include "adventofcode/cc/year2022/day15_shared.h"

#include <algorithm>
#include <cmath>
#include <list>
#include <sstream>
#include <string>
#include <vector>

#include "absl/container/flat_hash_set.h"
#include "absl/log/check.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/str_format.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"
#include "re2/re2.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day15_shared {
namespace {

// Span represents a closed-open span of integers [from, to). The default
// constructor results in an empty span with from = 0.
struct Span {
  int from;  // Inclusive.
  int to;    // Exclusive.

  std::string String() const { return absl::StrFormat("[%d,%d)", from, to); }

  // Size returns the number of integers covered by this span.
  int Size() const { return to - from; }
  // Contains returns whether this span contains the given integer.
  bool Contains(int x) const { return x >= from && x < to; }

  // Joinable returns true if lhs and rhs together form an unbroken span of
  // integers. For example, [0, 1) and [1, 3) are joinable to form the unbroken
  // span [0, 3).
  static bool Joinable(const Span& lhs, const Span& rhs) {
    // The only way for two spans to _not_ be joinable is if the smallest `from`
    // is larger than the largest `to`. In other words, if one of the spans
    // starts after the other has ended.
    //
    // We want to return true if they _are_ joinable, so we negate that logic.
    return !(std::min(lhs.from, rhs.from) > std::max(lhs.to, rhs.to));
  }
};

// SpanSet represents a dense set of integers as a set of unbroken spans of
// integers.
class SpanSet {
 public:
  // Add inserts the given span into the set.
  void Add(const Span& s) {
    // If the incoming span is empty, we do nothing.
    if (s.Size() == 0) {
      return;
    }
    // Special case: list is empty.
    if (spans_.empty()) {
      spans_.push_back(s);
      return;
    }
    // There is at least one span. We want to find a place to insert s so that
    // it can only possibly overlap with later spans, i.e., is guaranteed to not
    // overlap with earlier spans.
    auto iter = spans_.begin();
    while (true) {
      if (iter == spans_.end()) {
        break;
      }
      // If we know for sure that iter points to a span that cannot be joined
      // with s, we advance iter and continue looking. Note that this is
      // different from simply checking whether they are joinable. As an
      // example, consider a list containing [0, 1) and [6, 7), and we're trying
      // to insert [2, 3). It's not joinable with any of the existing spans, but
      // the order of spans must be contained. So we keep advancing forward in
      // the list as long as we know the new span cannot be inserted before the
      // span pointed to by iter.
      if (iter->to < s.from) {
        iter++;
        continue;
      }
      // s should be inserted before iter.
      break;
    }
    // iter now points to the first span that might be joinable with s. Insert s
    // before that, and start joining forward.
    auto cur = spans_.insert(iter, s);
    while (cur != spans_.end()) {
      auto next = cur;
      next++;
      if (next == spans_.end()) {
        // We have reached the end of the list; no more spans to join.
        break;
      }
      // If the current and next spans aren't joinable, we are done.
      if (!Span::Joinable(*cur, *next)) {
        break;
      }
      // Join the spans into cur and delete the next element.
      cur->from = std::min(cur->from, next->from);
      cur->to = std::max(cur->to, next->to);
      spans_.erase(next);
    }
  }

  // Spans returns the spans that this set is built from.
  std::vector<Span> Spans() const {
    return std::vector<Span>(spans_.begin(), spans_.end());
  }

  std::string String() const {
    std::stringstream buf;
    bool first = true;
    for (const Span& s : Spans()) {
      if (!first) {
        buf << " - ";
      }
      first = false;
      buf << s.String();
    }
    return buf.str();
  }

 private:
  std::list<Span> spans_;
};

struct Pos {
  int x;
  int y;

  template <typename H>
  friend H AbslHashValue(H h, const Pos& p) {
    return H::combine(std::move(h), p.x, p.y);
  }

  // NOLINTNEXTLINE
  friend inline bool operator==(const Pos& lhs, const Pos& rhs) {
    return lhs.x == rhs.x && lhs.y == rhs.y;
  }
};

struct Reading {
  Pos sensor;
  Pos beacon;

  static Reading Parse(absl::string_view line) {
    Reading r;
    CHECK(RE2::FullMatch(
        line,
        R"(Sensor at x=(-?\d+), y=(-?\d+): closest beacon is at x=(-?\d+), y=(-?\d+))",
        &r.sensor.x, &r.sensor.y, &r.beacon.x, &r.beacon.y));
    return r;
  }

  std::string String() const {
    return absl::StrFormat(
        "Sensor at x=%d, y=%d: closest beacon is at x=%d, y=%d", sensor.x,
        sensor.y, beacon.x, beacon.y);
  }

  // Radius is the radius of the circle that the sensor in this reading covers.
  // All positions within this radius is guaranteed to not contain a beacon
  // (except the one included in the reading).
  int Radius() const {
    return std::abs(sensor.x - beacon.x) + std::abs(sensor.y - beacon.y);
  }

  // Covers returns true if the given position is within the radius of the
  // sensor in this reading.
  bool Covers(const Pos& p) const {
    return std::abs(p.x - sensor.x) + std::abs(p.y - sensor.y) <= Radius();
  }

  // CoveredPositions returns the unbroken span of x coordinates at the given y
  // coordinate that are covered by the sensor in this reading. If no x
  // coordinates are covered, this function returns an empty Span (i.e., one
  // with Size() == 0).
  Span CoveredPositions(int target_y) const {
    // We can use a little bit of math to quickly calculate this.
    // The sensor forms a "circle" of positions it can see that don't contain a
    // beacon, like so (from the example):
    //
    //                1    1    2    2
    //      0    5    0    5    0    5
    // -2 ..........#.................
    // -1 .........###................
    //  0 ....S...#####...............
    //  1 .......#######........S.....
    //  2 ......#########S............
    //  3 .....###########SB..........
    //  4 ....#############...........
    //  5 ...###############..........
    //  6 ..#################.........
    //  7 .#########S#######S#........
    //  8 ..#################.........
    //  9 ...###############..........
    // 10 ....B############...........
    // 11 ..S..###########............
    // 12 ......#########.............
    // 13 .......#######..............
    // 14 ........#####.S.......S.....
    // 15 B........###................
    // 16 ..........#SB...............
    // 17 ................S..........B
    // 18 ....S.......................
    // 19 ............................
    // 20 ............S......S........
    // 21 ............................
    // 22 .......................B....
    //
    // This doesn't really look like a circle, but in taxicab geometry, it is.
    // The center point of the circle is the sensor position, and the radius is
    // the Manhattan distance from the sensor to the beacon.
    //
    // With a circle like this, and given a target y, we can calculate whether
    // this y is included in the circle, and if so, what x coordinates are
    // covered by the circle. Let C be the center of the circle, R be the
    // radius, and Y be the target. If Y is at a distance R from C, then the
    // overlap is 1. If Y is at a distance R-1 from C, then the overlap is 3.
    // And so on:
    //
    //         C_x
    //          v
    //   R .....#..... => 1
    // R-1 ....###.... => 3
    // R-2 ...#####... => 5
    //
    // We can see that the overlap spans the range [C_x-d, C_x+d] where d =
    // R-|Y-C_y|. This can easily be seen in the illustration above.
    //
    // If the radius isn't big enough to cover the target y, d will be a
    // negative value.
    int r = Radius();
    int d = r - std::abs(target_y - sensor.y);
    if (d < 0) {
      return Span{};  // Empty span.
    }
    Span s;
    s.from = sensor.x - d;
    s.to = sensor.x + d + 1;
    return s;
  }

  // ClosestUncovered returns all the closest positions that are not covered by
  // the sensor in this reading. All such positions are at a distance of
  // Radius() + 1 from the sensor.
  std::vector<Pos> ClosestUncovered(int xy_max) const {
    std::vector<Pos> uncovered;
    auto eligible = [&xy_max](const Pos& p) -> bool {
      return p.x >= 0 && p.x <= xy_max && p.y >= 0 && p.y <= xy_max;
    };
    int r = Radius();
    for (int i = 0; i <= r; i++) {
      int c_x = sensor.x;
      int c_y = sensor.y;
      // Up-right quadrant.
      if (Pos p{
              .x = c_x + i,
              .y = c_y + (r + 1 - i),
          };
          eligible(p)) {
        uncovered.push_back(p);
      }
      // Down-right quadrant.
      if (Pos p{
              .x = c_x + (r + 1 - i),
              .y = c_y - i,
          };
          eligible(p)) {
        uncovered.push_back(p);
      }
      // Down-left quadrant.
      if (Pos p{
              .x = c_x - i,
              .y = c_y - (r + 1 - i),
          };
          eligible(p)) {
        uncovered.push_back(p);
      }
      // Up-left quadrant.
      if (Pos p{
              .x = c_x - (r + 1 - i),
              .y = c_y + i,
          };
          eligible(p)) {
        uncovered.push_back(p);
      }
    }
    return uncovered;
  }
};

}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input, int target_y) {
  SpanSet covered;
  absl::flat_hash_set<int> beacon_xs;
  for (absl::string_view line :
       absl::StrSplit(input, '\n', absl::SkipEmpty())) {
    Reading r = Reading::Parse(line);
    covered.Add(r.CoveredPositions(target_y));
    if (r.beacon.y == target_y) {
      beacon_xs.insert(r.beacon.x);
    }
  }
  int impossible = 0;
  for (const Span& s : covered.Spans()) {
    impossible += s.Size();
    for (int x : beacon_xs) {
      if (s.Contains(x)) {
        impossible--;
      }
    }
  }
  return std::to_string(impossible);
}

absl::StatusOr<std::string> Part2(absl::string_view input, int xy_max) {
  std::vector<Reading> readings;
  absl::flat_hash_set<Pos> occupied;
  for (absl::string_view line :
       absl::StrSplit(input, '\n', absl::SkipEmpty())) {
    Reading r = Reading::Parse(line);
    readings.push_back(r);
    occupied.insert(r.sensor);
  }
  for (size_t i = 0; i < readings.size(); i++) {
    for (const Pos& p : readings[i].ClosestUncovered(xy_max)) {
      if (occupied.contains(p)) {
        continue;
      }
      bool covered = false;
      for (size_t j = 0; j < readings.size(); j++) {
        if (j == i) {
          continue;
        }
        if (readings[j].Covers(p)) {
          covered = true;
          break;
        }
      }
      if (!covered) {
        unsigned long long int ans = p.x * 4000000ull + p.y;
        return std::to_string(ans);
      }
    }
  }
  return absl::InternalError("no answer found");
}

}  // namespace day15_shared
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
