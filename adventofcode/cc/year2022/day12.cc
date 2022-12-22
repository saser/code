#include "adventofcode/cc/year2022/day12.h"

// Optimization ideas:
// * Use A*. For part 2, h[node] could be "Manhattan distance to closest 'a'
//   node".
// * Use Jump Point Search
//   (https://harablog.wordpress.com/2011/09/07/jump-point-search/), which can
//   supposedly speed up A* quite significantly in grids with lots of open
//   spaces (as there are in my input).

#include <queue>
#include <sstream>
#include <string>
#include <vector>

#include "absl/container/flat_hash_map.h"
#include "absl/container/flat_hash_set.h"
#include "absl/status/statusor.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day12 {
namespace {

class Grid {
 public:
  Grid() = delete;
  Grid(size_t rows, size_t cols)
      : data_(rows * cols), rows_(rows), cols_(cols) {}

  size_t idx(size_t row, size_t col) const { return row * cols_ + col; }
  char operator[](size_t idx) const { return data_[idx]; }
  char& operator[](size_t idx) { return data_[idx]; }

  char elevation(size_t idx) const {
    char c = (*this)[idx];
    switch (c) {
      case 'S':
        return 'a';
      case 'E':
        return 'z';
      default:
        return c;
    }
  }

  size_t rows() const { return rows_; }
  size_t cols() const { return cols_; }

  size_t start() const {
    for (size_t i = 0; i < data_.size(); i++) {
      if (data_[i] == 'S') {
        return i;
      }
    }
    return -1;
  }

  size_t end() const {
    for (size_t i = 0; i < data_.size(); i++) {
      if (data_[i] == 'E') {
        return i;
      }
    }
    return -1;
  }

  std::string String() const {
    std::stringstream buf;
    for (size_t row = 0; row < rows(); row++) {
      if (row > 0) {
        buf << std::endl;
      }
      for (size_t col = 0; col < cols(); col++) {
        buf << (*this)[idx(row, col)];
      }
    }
    return buf.str();
  }

 private:
  std::vector<char> data_;
  size_t rows_;
  size_t cols_;
};

Grid parse2(absl::string_view input) {
  std::vector<absl::string_view> lines =
      absl::StrSplit(input, '\n', absl::SkipEmpty());
  size_t rows = lines.size();
  size_t cols = lines[0].size();
  Grid g(rows, cols);
  for (size_t row = 0; row < g.rows(); row++) {
    for (size_t col = 0; col < g.cols(); col++) {
      g[g.idx(row, col)] = lines[row][col];
    }
  }
  return g;
}

class Graph {
 public:
  Graph() = delete;
  Graph(const Grid& grid) {
    for (size_t row = 0; row < grid.rows(); row++) {
      for (size_t col = 0; col < grid.cols(); col++) {
        size_t current = grid.idx(row, col);
        auto reachable = [&grid, &current](const size_t& neighbor) -> bool {
          return grid.elevation(neighbor) <= grid.elevation(current) + 1;
        };
        std::vector<size_t> adjacent;
        if (row > 0) {
          adjacent.push_back(grid.idx(row - 1, col));
        }
        if (row < grid.rows() - 1) {
          adjacent.push_back(grid.idx(row + 1, col));
        }
        if (col > 0) {
          adjacent.push_back(grid.idx(row, col - 1));
        }
        if (col < grid.cols() - 1) {
          adjacent.push_back(grid.idx(row, col + 1));
        }
        for (size_t neighbor : adjacent) {
          if (reachable(neighbor)) {
            out_[current].insert(neighbor);
            in_[neighbor].insert(current);
          }
        }
      }
    }
  }

  void Reverse() { out_.swap(in_); }

  const absl::flat_hash_set<size_t>& Neighbors(const size_t& idx) const {
    return out_.at(idx);
  }

 private:
  absl::flat_hash_map<size_t, absl::flat_hash_set<size_t>> out_;
  absl::flat_hash_map<size_t, absl::flat_hash_set<size_t>> in_;
};

class Dijkstra {
 public:
  Dijkstra() = delete;
  Dijkstra(const Graph& g, size_t start, absl::flat_hash_set<size_t> targets)
      : graph_(g), start_(start), targets_(targets) {}

  size_t Run() const {
    absl::flat_hash_map<size_t, size_t> shortest;
    auto less = [&shortest](size_t i1, size_t i2) -> bool {
      return shortest[i1] > shortest[i2];
    };
    std::priority_queue<size_t, std::vector<size_t>, decltype(less)> q(less);
    shortest[start_] = 0;
    q.push(start_);
    while (!q.empty()) {
      size_t idx = q.top();
      q.pop();
      if (targets_.contains(idx)) {
        return shortest[idx];
      }
      for (size_t neighbor : graph_.Neighbors(idx)) {
        if (shortest.contains(neighbor)) {
          continue;
        }
        shortest[neighbor] = shortest[idx] + 1;
        q.push(neighbor);
      }
    }
    return -1;  // Not found (this will actually be a large number).
  }

 private:
  const Graph& graph_;
  size_t start_;
  absl::flat_hash_set<size_t> targets_;
};

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  Grid grid = parse2(input);
  Graph graph(grid);
  size_t start;
  absl::flat_hash_set<size_t> targets;
  if (part1) {
    start = grid.start();
    targets.insert(grid.end());
  } else {
    graph.Reverse();
    start = grid.end();
    for (size_t row = 0; row < grid.rows(); row++) {
      for (size_t col = 0; col < grid.cols(); col++) {
        size_t idx = grid.idx(row, col);
        if (grid[idx] == 'a') {
          targets.insert(idx);
        }
      }
    }
  }
  Dijkstra solution(graph, start, targets);
  return std::to_string(solution.Run());
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}

}  // namespace day12
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
