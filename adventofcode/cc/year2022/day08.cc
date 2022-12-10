#include "adventofcode/cc/year2022/day08.h"

#include <algorithm>
#include <sstream>
#include <string>
#include <vector>

#include "absl/status/statusor.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day08 {
namespace {

// Grid<T> represents a two-dimensional grid of elements of type T.
template <typename T>
class Grid {
 public:
  Grid() = delete;
  // Grid(rows, cols) initializes a grid with the given number of rows and
  // columns. The elements of the grid will be the "zero-value" of T.
  Grid(size_t rows, size_t cols) : n_rows_(rows), n_cols_(cols) {
    data2_ = std::vector<T>(n_rows_ * n_cols_);
  }

  // Get returns the value at the given row and column.
  T Get(size_t row, size_t col) const { return data2_.at(row * n_cols_ + col); }

  // Set sets the value at the given row and column.
  void Set(size_t row, size_t col, T v) { data2_.at(row * n_cols_ + col) = v; }

  // NRows returns the number of rows in the grid.
  size_t NRows() const { return n_rows_; }

  // NCols return the number of columns in the grid.
  size_t NCols() const { return n_cols_; }

  // String returns a string representation of the grid.
  std::string String() const {
    std::stringstream buf;
    for (size_t row = 0; row < NRows(); row++) {
      if (row > 0) {
        buf << std::endl;
      }
      for (size_t col = 0; col < NCols(); col++) {
        buf << Get(row, col);
      }
    }
    return buf.str();
  }

 private:
  // data2_ holds the contents of the grid. The first n_cols_ elements are for
  // row 0; the next n_cols_ elements are for row 1; and so on.
  std::vector<T> data2_;
  size_t n_rows_;
  size_t n_cols_;
};

Grid<int> parse(absl::string_view input) {
  std::vector<absl::string_view> lines =
      absl::StrSplit(input, '\n', absl::SkipEmpty());
  size_t rows = lines.size();
  size_t cols = lines[0].size();
  Grid<int> g(rows, cols);
  for (size_t row = 0; row < rows; row++) {
    for (size_t col = 0; col < cols; col++) {
      g.Set(row, col, lines[row][col] - '0');
    }
  }
  return g;
}

}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  Grid<int> g = parse(input);
  size_t n_rows = g.NRows();
  size_t n_cols = g.NCols();
  Grid<bool> visible(n_rows, n_cols);
  int n_visible = 0;

  // The basic idea of my algorithm is this:
  // 1. Go through each row and mark all trees that are visible from the left or
  //    the right of the grid.
  // 2. Go through each column and mark all trees that are visible from the top
  //    or the bottom of the grid.
  // 3. Take the size of the union of these trees.

  // Step 1: go through the rows one by one.
  for (size_t row = 0; row < n_rows; row++) {
    // Go through left-to-right first.
    {
      int max = -1;  // The maximum height we've seen so far.
      for (size_t col = 0; col < n_cols; col++) {
        int height = g.Get(row, col);
        if (height > max) {
          if (!visible.Get(row, col)) {
            // We encountered a visible tree we haven't seen before.
            n_visible++;
          }
          visible.Set(row, col, true);
          max = height;
        }
      }
    }

    // Then go through right-to-left.
    {
      int max = -1;  // The maximum height we've seen so far.
      for (size_t n = 0; n < n_cols; n++) {
        size_t col = (n_cols - 1) - n;
        int height = g.Get(row, col);
        if (height > max) {
          if (!visible.Get(row, col)) {
            // We encountered a visible tree we haven't seen before.
            n_visible++;
          }
          visible.Set(row, col, true);
          max = height;
        }
      }
    }
  }

  // Step 2: go through the columns one by one.
  for (size_t col = 0; col < n_cols; col++) {
    // Go through top-to-bottom first.
    {
      int max = -1;  // The maximum height we've seen so far.
      for (size_t row = 0; row < n_rows; row++) {
        int height = g.Get(row, col);
        if (height > max) {
          if (!visible.Get(row, col)) {
            // We encountered a visible tree we haven't seen before.
            n_visible++;
          }
          visible.Set(row, col, true);
          max = height;
        }
      }
    }

    // Then go through bottom-to-top.
    {
      int max = -1;  // The maximum height we've seen so far.
      for (size_t n = 0; n < n_rows; n++) {
        size_t row = (n_rows - 1) - n;
        int height = g.Get(row, col);
        if (height > max) {
          if (!visible.Get(row, col)) {
            // We encountered a visible tree we haven't seen before.
            n_visible++;
          }
          visible.Set(row, col, true);
          max = height;
        }
      }
    }
  }
  return std::to_string(n_visible);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  Grid<int> g = parse(input);
  size_t n_rows = g.NRows();
  size_t n_cols = g.NCols();

  // Left.
  Grid<int> left(n_rows, n_cols);
  for (size_t row = 0; row < n_rows; row++) {
    for (size_t i = 0; i < n_cols; i++) {
      size_t col = i;
      int h = g.Get(row, col);
      int score = 0;
      if (i > 0) {
        for (size_t j = 1; j <= i; j++) {
          score++;
          if (g.Get(row, col - j) >= h) {
            break;
          }
        }
      }
      left.Set(row, col, score);
    }
  }

  // Right.
  Grid<int> right(n_rows, n_cols);
  for (size_t row = 0; row < n_rows; row++) {
    for (size_t i = 0; i < n_cols; i++) {
      size_t col = (n_cols - 1) - i;
      int h = g.Get(row, col);
      int score = 0;
      if (i > 0) {
        for (size_t j = 1; j <= i; j++) {
          score++;
          if (g.Get(row, col + j) >= h) {
            break;
          }
        }
      }
      right.Set(row, col, score);
    }
  }

  // Up.
  Grid<int> up(n_rows, n_cols);
  for (size_t col = 0; col < n_cols; col++) {
    for (size_t i = 0; i < n_rows; i++) {
      size_t row = i;
      int h = g.Get(row, col);
      int score = 0;
      if (i > 0) {
        for (size_t j = 1; j <= i; j++) {
          score++;
          if (g.Get(row - j, col) >= h) {
            break;
          }
        }
      }
      up.Set(row, col, score);
    }
  }

  // Down.
  Grid<int> down(n_rows, n_cols);
  for (size_t col = 0; col < n_cols; col++) {
    for (size_t i = 0; i < n_rows; i++) {
      size_t row = (n_rows - 1) - i;
      int h = g.Get(row, col);
      int score = 0;
      if (i > 0) {
        for (size_t j = 1; j <= i; j++) {
          score++;
          if (g.Get(row + j, col) >= h) {
            break;
          }
        }
      }
      down.Set(row, col, score);
    }
  }

  // Now we have all the scenic scores in all directions; lets find the biggest
  // product.
  int max = -1;
  for (size_t row = 0; row < n_rows; row++) {
    for (size_t col = 0; col < n_cols; col++) {
      int score = left.Get(row, col) * right.Get(row, col) * up.Get(row, col) *
                  down.Get(row, col);
      max = std::max(max, score);
    }
  }
  return std::to_string(max);
}

}  // namespace day08
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
