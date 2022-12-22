#include "adventofcode/cc/year2022/day18.h"

#include <algorithm>
#include <cmath>
#include <queue>
#include <string>

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
namespace day18 {
namespace {

struct Pos3 {
  int x;
  int y;
  int z;

  static Pos3 Parse(absl::string_view line) {
    Pos3 p;
    std::vector<absl::string_view> parts = absl::StrSplit(line, ',');
    CHECK(absl::SimpleAtoi(parts[0], &p.x)) << parts[0];
    CHECK(absl::SimpleAtoi(parts[1], &p.y)) << parts[1];
    CHECK(absl::SimpleAtoi(parts[2], &p.z)) << parts[2];
    return p;
  }

  static int Distance(const Pos3& lhs, const Pos3& rhs) {
    return std::abs(lhs.x - rhs.x) + std::abs(lhs.y - rhs.y) +
           std::abs(lhs.z - rhs.z);
  }

  std::string String() const { return absl::StrFormat("%d,%d,%d", x, y, z); }

  // NOLINTNEXTLINE
  friend bool operator==(const Pos3& lhs, const Pos3& rhs) {
    return lhs.x == rhs.x && lhs.y == rhs.y && lhs.z == rhs.z;
  }

  template <typename H>
  friend H AbslHashValue(H h, const Pos3& p) {
    return H::combine(std::move(h), p.x, p.y, p.z);
  }
};

static inline constexpr Pos3 kDeltas[6] = {
    Pos3{.x = +1, .y = 0, .z = 0}, Pos3{.x = -1, .y = 0, .z = 0},
    Pos3{.x = 0, .y = +1, .z = 0}, Pos3{.x = 0, .y = -1, .z = 0},
    Pos3{.x = 0, .y = 0, .z = +1}, Pos3{.x = 0, .y = 0, .z = -1},
};

int DirectedEdgeCount(absl::flat_hash_set<Pos3> cubes) {
  CHECK_GT(cubes.size(), 0) << "The set of cubes was empty";

  absl::flat_hash_set<Pos3> visited;
  int edges = 0;
  for (Pos3 cube : cubes) {
    if (visited.contains(cube)) {
      continue;
    }
    visited.insert(cube);
    for (Pos3 d : kDeltas) {
      Pos3 next{
          .x = cube.x + d.x,
          .y = cube.y + d.y,
          .z = cube.z + d.z,
      };
      if (!cubes.contains(next)) {
        continue;
      }
      edges++;
    }
  }
  return edges;
}

int ExteriorSurfaceArea(absl::flat_hash_set<Pos3> cubes) {
  // Idea:
  // 1. Build a bounding box around the cubes.
  // 2. Increase the bounding box edges by 1. This leaves a "layer" of free
  //    space outside the cube.
  // 3. Use a graph search of the free space. Everytime a free cube shares a
  //    side with a concrete cube, we increase the surface area by 1.

  auto x_cmp = [](Pos3 lhs, Pos3 rhs) -> bool { return lhs.x < rhs.x; };
  auto x_edges = std::minmax_element(cubes.cbegin(), cubes.cend(), x_cmp);
  auto y_cmp = [](Pos3 lhs, Pos3 rhs) -> bool { return lhs.y < rhs.y; };
  auto y_edges = std::minmax_element(cubes.cbegin(), cubes.cend(), y_cmp);
  auto z_cmp = [](Pos3 lhs, Pos3 rhs) -> bool { return lhs.z < rhs.z; };
  auto z_edges = std::minmax_element(cubes.cbegin(), cubes.cend(), z_cmp);

  // These variables define the bounding box.
  int x_min = x_edges.first->x - 1, x_max = x_edges.second->x + 1,
      y_min = y_edges.first->y - 1, y_max = y_edges.second->y + 1,
      z_min = z_edges.first->z - 1, z_max = z_edges.second->z + 1;

  std::queue<Pos3> q;
  q.push(Pos3{.x = x_min, .y = y_min, .z = z_min});
  absl::flat_hash_set<Pos3> visited;
  int area = 0;
  while (!q.empty()) {
    Pos3 cube = q.front();
    q.pop();
    if (visited.contains(cube)) {
      continue;
    }
    visited.insert(cube);
    for (Pos3 d : kDeltas) {
      Pos3 next{
          .x = cube.x + d.x,
          .y = cube.y + d.y,
          .z = cube.z + d.z,
      };
      if ((next.x < x_min || next.x > x_max) ||
          (next.y < y_min || next.y > y_max) ||
          (next.z < z_min || next.z > z_max)) {
        continue;
      }
      if (cubes.contains(next)) {
        area++;
        continue;
      }
      q.push(next);
    }
  }

  return area;
}

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  absl::flat_hash_set<Pos3> cubes;
  for (absl::string_view line :
       absl::StrSplit(input, '\n', absl::SkipEmpty())) {
    cubes.insert(Pos3::Parse(line));
  }
  int surface_area;
  if (part1) {
    surface_area = 6 * cubes.size() - DirectedEdgeCount(cubes);
  } else {
    surface_area = ExteriorSurfaceArea(cubes);
  }
  return std::to_string(surface_area);
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}
}  // namespace day18
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
