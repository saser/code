#include "adventofcode/cc/year2022/day22.h"

#include <algorithm>
#include <cmath>
#include <cstdint>
#include <optional>
#include <string>
#include <tuple>
#include <utility>
#include <vector>

#include "absl/container/flat_hash_map.h"
#include "absl/log/check.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_format.h"
#include "absl/strings/str_join.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"
#include "adventofcode/cc/geometry/pos.h"
#include "adventofcode/cc/trim.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day22 {
namespace {

// This code is long and messy. The main reason is that I wrote the solution for
// part 1 first, then the solution for part 2 became much more complicated so I
// created a bunch of extra code for it, and then I struggled with weird
// non-deterministic bugs for a long time and ended up without the energy to
// refactor this code and make it look nicer. To any reader (including future
// me): sorry.
//
// Also, the idea for part 2 came from /r/adventofcode on Reddit. It's very
// elegant, and I don't think my implementation of it does it justice. It can be
// found here:
// https://www.reddit.com/r/adventofcode/comments/zsct8w/comment/j18dzaa

using Pos2 = adventofcode::cc::geometry::Pos;

struct Vec3 {
  int64_t x, y, z;

  std::string String() const { return absl::StrFormat("(%d,%d,%d)", x, y, z); }

  template <typename H>
  friend H AbslHashValue(H h, const Vec3& p) {
    return H::combine(std::move(h), p.x, p.y, p.z);
  }

  friend bool operator==(const Vec3& lhs, const Vec3& rhs) {
    return lhs.x == rhs.x && lhs.y == rhs.y && lhs.z == rhs.z;
  }

  // Arithmetic operators taken from https://stackoverflow.com/a/52377719.
  Vec3& operator+=(const Vec3& rhs) {
    x += rhs.x;
    y += rhs.y;
    z += rhs.z;
    return *this;
  }
  friend Vec3 operator+(Vec3 lhs, const Vec3& rhs) {
    lhs += rhs;
    return lhs;
  }

  Vec3& operator-=(const Vec3& rhs) {
    x -= rhs.x;
    y -= rhs.y;
    z -= rhs.z;
    return *this;
  }

  Vec3& operator*=(int64_t f) {
    x *= f;
    y *= f;
    z *= f;
    return *this;
  }
  friend Vec3 operator*(Vec3 lhs, int64_t f) {
    lhs *= f;
    return lhs;
  }
};

class Map3 {
 public:
  static Map3 Parse(absl::string_view input) {
    Map3 m{};
    m.map_ = absl::StrSplit(input, '\n', absl::SkipEmpty());
    m.start_.y = 0;
    for (int64_t x = 0; x < m.map_[m.start_.y].size(); x++) {
      if (m.map_[m.start_.y][x] == '.') {
        m.start_.x = x;
        break;
      }
    }
    // We know that the input consists of 6 tiles. We also know that each tile
    // consists of the same number of '#' and '.' characters. We calculate the
    // total number of '#' and '.' in the entire input, divide by 6, and get the
    // number area of the tile. Since the tile is quadratic we simply take the
    // square root of the area to get the length of the side.
    size_t total_area =
        std::count_if(input.cbegin(), input.cend(),
                      [](char c) -> bool { return c == '.' || c == '#'; });
    size_t tile_area = total_area / 6;
    m.side_length_ = std::sqrt(tile_area);
    return m;
  }

  Pos2 Start() const { return start_; }

  bool InBounds(int64_t i, int64_t j) const {
    return i >= 0 && i < map_.size()        // In bounds vertically?
           && j >= 0 && j < map_[i].size()  // In bounds horizontally?
           && map_[i][j] != ' '             // Not empty space?
        ;
  }

  char At(int64_t i, int64_t j) const {
    CHECK(InBounds(i, j)) << "(" << i << "," << j << ") is out of bounds";
    return map_[i][j];
  }

  int64_t SideLength() const { return side_length_; }

  std::vector<std::string> Map() const {
    std::vector<std::string> map(map_.cbegin(), map_.cend());
    return map;
  }

 private:
  Map3() = default;

  // The map part of the input, split into lines. This contains the exact
  // representation of the input, meaning that the lines likely has different
  // lengths.
  std::vector<absl::string_view> map_;
  // The start position.
  Pos2 start_;
  // The side of the tiles.
  int64_t side_length_;
};

struct Map {
  std::vector<std::string> map;
  Pos2 start;

  static Map Parse(absl::string_view input) {
    Map m;
    m.map = absl::StrSplit(input, '\n', absl::SkipEmpty());
    // Expand the map so that it is rectangular, by padding with spaces -- it
    // makes the code for moving around much simpler.
    size_t width = 0;
    for (absl::string_view line : m.map) {
      width = std::max(width, line.size());
    }
    for (std::string& line : m.map) {
      if (line.size() < width) {
        line.append(width - line.size(), ' ');
      }
    }

    m.start.y = 0;
    absl::string_view top = m.map[0];
    for (size_t x = 0; x < top.size(); x++) {
      if (top[x] == '.') {
        m.start.x = x;
        break;
      }
    }
    return m;
  }
};

struct Instruction {
  uint8_t n;
  // The very last instruction won't have a turn.
  std::optional<bool> clockwise;

  static Instruction Parse(absl::string_view s) {
    Instruction i;
    // n will always fit in 8 bits, but absl::SimpleAtoi only works on 32- and
    // 64-bit integers.
    uint32_t n;
    if (char c = s.back(); c == 'L' || c == 'R') {
      i.clockwise = c == 'R';
      absl::string_view n_str = s.substr(0, s.size() - 1);
      CHECK(absl::SimpleAtoi(n_str, &n))
          << "'" << n_str << "' from '" << s << "'";
    } else {
      CHECK(absl::SimpleAtoi(s, &n)) << "'" << s << "' from '" << s << "'";
    }
    i.n = n;
    return i;
  }

  std::string String() const {
    std::string s;
    s += std::to_string(n);
    if (clockwise.has_value()) {
      s += *clockwise ? 'R' : 'L';
    }
    return s;
  }
};

std::vector<Instruction> ParseInstructions(absl::string_view input) {
  input = adventofcode::cc::trim::TrimSpace(input);
  std::vector<Instruction> instructions;
  size_t start = 0;
  while (start < input.size()) {
    size_t end = input.find_first_of("LR", start);
    if (end == absl::string_view::npos) {
      end = input.size();
    } else {
      end += 1;  // -> point to next digit after 'L' or 'R'.
    }
    instructions.push_back(
        Instruction::Parse(input.substr(start, end - start)));
    start = end;
  }
  return instructions;
}

struct State {
  enum class Direction { kRight, kDown, kLeft, kUp };
  Pos2 pos;
  Direction dir;

  uint32_t Password() const {
    uint32_t password = 1000 * (pos.y + 1) + 4 * (pos.x + 1);
    uint32_t facing;
    switch (dir) {
      case Direction::kRight:
        facing = 0;
        break;
      case Direction::kDown:
        facing = 1;
        break;
      case Direction::kLeft:
        facing = 2;
        break;
      case Direction::kUp:
        facing = 3;
        break;
    }
    password += facing;
    return password;
  }

  void Apply(const Map& map, const Instruction& instruction) {
    for (uint8_t n = 0; n < instruction.n; n++) {
      bool moved = Step(map);
      if (!moved) {
        break;
      }
    }
    if (instruction.clockwise.has_value()) {
      if (*instruction.clockwise) {
        switch (dir) {
          case Direction::kRight:
            dir = Direction::kDown;
            break;
          case Direction::kDown:
            dir = Direction::kLeft;
            break;
          case Direction::kLeft:
            dir = Direction::kUp;
            break;
          case Direction::kUp:
            dir = Direction::kRight;
            break;
        }
      } else {
        switch (dir) {
          case Direction::kRight:
            dir = Direction::kUp;
            break;
          case Direction::kDown:
            dir = Direction::kRight;
            break;
          case Direction::kLeft:
            dir = Direction::kDown;
            break;
          case Direction::kUp:
            dir = Direction::kLeft;
            break;
        }
      }
    }
  }

  bool Step(const Map& map) {
    switch (dir) {
      case Direction::kRight:
        return StepRight(map);
        break;
      case Direction::kDown:
        return StepDown(map);
        break;
      case Direction::kLeft:
        return StepLeft(map);
        break;
      case Direction::kUp:
        return StepUp(map);
        break;
    }
  }

  bool StepRight(const Map& map) {
    absl::string_view line = map.map[pos.y];
    size_t x = (pos.x + 1) % line.size();
    while (line[x] == ' ') {
      x = (x + 1) % line.size();
    }
    if (line[x] == '.') {
      pos.x = x;
      return true;
    } else {
      return false;
    }
  }

  bool StepDown(const Map& map) {
    size_t y = (pos.y + 1) % map.map.size();
    while (map.map[y][pos.x] == ' ') {
      y = (y + 1) % map.map.size();
    }
    if (map.map[y][pos.x] == '.') {
      pos.y = y;
      return true;
    } else {
      return false;
    }
  }

  bool StepLeft(const Map& map) {
    absl::string_view line = map.map[pos.y];
    size_t x = (pos.x > 0) ? pos.x - 1 : line.size() - 1;
    while (line[x] == ' ') {
      x = (x > 0) ? x - 1 : line.size() - 1;
    }
    if (line[x] == '.') {
      pos.x = x;
      return true;
    } else {
      return false;
    }
  }

  bool StepUp(const Map& map) {
    size_t y = (pos.y > 0) ? pos.y - 1 : map.map.size() - 1;
    while (map.map[y][pos.x] == ' ') {
      y = (y > 0) ? y - 1 : map.map.size() - 1;
    }
    if (map.map[y][pos.x] == '.') {
      pos.y = y;
      return true;
    } else {
      return false;
    }
  }
};

// I might not always use this function, but it's good to keep around.
// NOLINTNEXTLINE
std::string DebugString(const Map& map, const State& state) {
  std::vector<std::string> lines = map.map;
  char dir;
  switch (state.dir) {
    case State::Direction::kRight:
      dir = '>';
      break;
    case State::Direction::kDown:
      dir = 'v';
      break;
    case State::Direction::kLeft:
      dir = '<';
      break;
    case State::Direction::kUp:
      dir = '^';
      break;
  }
  lines[state.pos.y][state.pos.x] = dir;
  return absl::StrJoin(lines, "\n");
}

Vec3 Cross(const Vec3& a, const Vec3& b) {
  return Vec3{
      .x = a.y * b.z - a.z * b.y,
      .y = a.z * b.x - a.x * b.z,
      .z = a.x * b.y - a.y * b.x,
  };
}

int64_t Dot(const Vec3& a, const Vec3& b) {
  return a.x * b.x + a.y * b.y + a.z * b.z;
}

Vec3 Normal(const Vec3& di, const Vec3& dj) { return Cross(di, dj); }

struct CubeMapping {
 public:
  // Mapping from position in the input to the 3D position on the cube and the
  // normal (defined as two orthogonal vectors) defining the face of the cube.
  absl::flat_hash_map<std::tuple<int64_t, int64_t>,  // (i, j)
                      std::tuple<Vec3, Vec3, Vec3>>  // (xyz, di, dj)
      faces;
  // Mapping from 3D position of the cube and the normal defining the face, to
  // the position in the input.
  absl::flat_hash_map<std::tuple<Vec3, Vec3>,        // (xyz, normal)
                      std::tuple<int64_t, int64_t>>  // (i, j)
      edges;

  CubeMapping() = default;
  static CubeMapping From(const Map3& map) {
    CubeMapping cm;
    int64_t i = map.Start().y;
    int64_t j = map.Start().x;
    Vec3 xyz{.x = 0, .y = 0, .z = 0};
    Vec3 di{.x = 0, .y = 1, .z = 0};
    Vec3 dj{.x = 1, .y = 0, .z = 0};
    cm.Discover(map, i, j, xyz, di, dj);
    return cm;
  }

 private:
  void Discover(const Map3& map,       // The map.
                int64_t i, int64_t j,  // 2D position in input.
                const Vec3& xyz,       // 3D position in cube.
                const Vec3& di,        // Positive i direction ("down").
                const Vec3& dj         // Positive j diredtion ("right").
  ) {
    if (!map.InBounds(i, j)) {
      return;
    }
    if (faces.contains({i, j})) {
      return;
    }
    faces[{i, j}] = {xyz, di, dj};
    int64_t s = map.SideLength();
    Vec3 n = Normal(di, dj);
    for (int64_t r = 0; r < s; r++) {
      // Left edge.
      edges[{xyz + di * r, n}] = {i + r, j};
      // Right edge.
      edges[{xyz + di * r + dj * (s - 1), n}] = {i + r, j + s - 1};
      // Top edge.
      edges[{xyz + dj * r, n}] = {i, j + r};
      // Bottom edge.
      edges[{xyz + di * (s - 1) + dj * r, n}] = {i + s - 1, j + r};
    }
    // Cross left edge.
    Discover(map, i, j - s, xyz + Cross(dj, di) * (s - 1), di, Cross(di, dj));
    // Cross right edge.
    Discover(map, i, j + s, xyz + dj * (s - 1), di, Cross(dj, di));
    // Cross bottom edge.
    Discover(map, i + s, j, xyz + di * (s - 1), Cross(dj, di), dj);
    // Cross top edge.
    Discover(map, i - s, j, xyz + Cross(dj, di) * (s - 1), Cross(di, dj), dj);
  }
};

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  if (!part1) {
    return absl::UnimplementedError("not implemented");
  }
  std::vector<absl::string_view> parts = absl::StrSplit(input, "\n\n");
  Map map = Map::Parse(parts[0]);
  std::vector<Instruction> instructions = ParseInstructions(parts[1]);
  State s{
      .pos = map.start,
      .dir = State::Direction::kRight,
  };
  for (const Instruction& instruction : instructions) {
    s.Apply(map, instruction);
  }
  return std::to_string(s.Password());
}

class State3 {
 public:
  State3(const Map3& map, const CubeMapping& cm)
      : i_(map.Start().y),
        j_(map.Start().x),
        di_(0),
        dj_(1),
        map_(map),
        cm_(cm) {}

  int64_t Password() const {
    int64_t password = (i_ + 1) * 1000 + (j_ + 1) * 4;
    if (di_ == 0 && dj_ == 1) {  // Right.
      password += 0;
    } else if (di_ == 1 && dj_ == 0) {  // Down.
      password += 1;
    } else if (di_ == 0 && dj_ == -1) {  // Left.
      password += 2;
    } else if (di_ == -1 && dj_ == 0) {  // Up.
      password += 3;
    }
    return password;
  }

  void Apply(const Instruction& instr) {
    for (uint8_t n = 0; n < instr.n; n++) {
      bool moved = Step();
      if (!moved) {
        break;
      }
    }
    if (instr.clockwise.has_value()) {
      if (instr.clockwise.value()) {
        // di, dj = dj, -di
        std::swap(di_, dj_);
        dj_ *= -1;
      } else {
        // di, dj = -dj, di
        std::swap(di_, dj_);
        di_ *= -1;
      }
    }
  }

  bool Step() {
    int64_t i_new = i_ + di_;
    int64_t j_new = j_ + dj_;
    int64_t di_new = di_;
    int64_t dj_new = dj_;

    if (!map_.InBounds(i_new, j_new)) {
      int64_t s = map_.SideLength();
      // Find what face of the cube we were on before going out of bounds. Do
      // this by "normalizing" (i, j) to the top left of the cube face.
      auto [xyz, di3, dj3] = cm_.faces.at({(i_ / s) * s, (j_ / s) * s});
      // Then find the 3D position we're currently occupying.
      Vec3 here = xyz + di3 * (i_ % s) + dj3 * (j_ % s);
      // Look up the new positions in the input based on the new face we will be
      // occupying, defined by the normal. The expression (di3*di_ + dj3*dj_)
      // will "select" exactly one of di3 and dj3 to use as the normal,
      // potentially inverting them as well. It's difficult to explain why this
      // works, but it does.
      auto [ii, jj] = cm_.edges.at({here, di3 * di_ + dj3 * dj_});
      i_new = ii;
      j_new = jj;
      // Now that we know the new position, we need to calculate the new
      // direction.
      std::tuple<Vec3, Vec3, Vec3> f =
          cm_.faces.at({(i_new / s) * s, (j_new / s) * s});
      Vec3 n = Normal(di3, dj3);
      di3 = std::get<1>(f);
      dj3 = std::get<2>(f);
      // I'm not entirely sure why this is "-Dot(...)" rather than "Dot(...)"
      // but it works...
      di_new = -Dot(di3, n);
      dj_new = -Dot(dj3, n);
    }

    if (map_.At(i_new, j_new) == '#') {
      // The tile we would end up on is a wall, so we didn't move.
      return false;
    }
    // We did move.
    i_ = i_new;
    j_ = j_new;
    di_ = di_new;
    dj_ = dj_new;
    return true;
  }

  // I might not always use this function, but it's good to keep around.
  // NOLINTNEXTLINE
  std::string DebugString() const {
    std::vector<std::string> lines = map_.Map();
    char c = 'x';
    if (di_ == 0 && dj_ == 1) {  // Right.
      c = '>';
    } else if (di_ == 1 && dj_ == 0) {  // Down.
      c = 'v';
    } else if (di_ == 0 && dj_ == -1) {  // Left.
      c = '<';
    } else if (di_ == -1 && dj_ == 0) {  // Up.
      c = '^';
    }
    lines[i_][j_] = c;
    return absl::StrJoin(lines, "\n");
  }

 private:
  int64_t i_, j_;    // 2D position in the input. i = row, j = col.
  int64_t di_, dj_;  // 2D direction relative to input. Exactly one of these
                     // will be 1; the other will be 0.
  const Map3& map_;
  const CubeMapping& cm_;
};

}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  std::vector<absl::string_view> parts = absl::StrSplit(input, "\n\n");
  Map3 map = Map3::Parse(parts[0]);
  CubeMapping cm = CubeMapping::From(map);
  State3 s(map, cm);
  std::vector<Instruction> instructions = ParseInstructions(parts[1]);
  for (const Instruction& instruction : instructions) {
    s.Apply(instruction);
  }
  return std::to_string(s.Password());
  return solve(input, /*part1=*/false);
}
}  // namespace day22
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
