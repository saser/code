#ifndef ADVENTOFCODE_CC_GEOMETRY_POS_H
#define ADVENTOFCODE_CC_GEOMETRY_POS_H

#include <cstdint>
#include <ostream>

#include "absl/strings/str_format.h"

namespace adventofcode {
namespace cc {
namespace geometry {
// Pos represents a point with integral coordinates in a 2D plane.
struct Pos {
  int64_t x, y;

  // Distance returns the Manhattan distance between this Pos and another Pos.
  // Calling Distance with no argument returns the Manhattan distance to (0, 0).
  // I have not bothered to deal with edge cases like underflows or the distance
  // being more than 2^63-1.
  int64_t Distance() const;
  int64_t Distance(const Pos& to) const;

  // AbslStringify implements string formatting for integration with Abseil
  // libraries.
  template <typename Sink>
  friend void AbslStringify(Sink& sink, const Pos& p) {
    absl::Format(&sink, "(%d,%d)", p.x, p.y);
  }

  // GoogleTest _should_ be able to make use of AbslStringify, but for some
  // reason it doesn't. However, it does pick up the PrintTo function, as
  // explained in
  // https://github.com/google/googletest/blob/3288c4deae0710464a0fd21316084c408798b960/docs/advanced.md#teaching-googletest-how-to-print-your-values.
  // This implementation of PrintTo simply calls out to the AbslStringify
  // function, so that they share the implementation.
  friend void PrintTo(const Pos& p, std::ostream* os) { AbslStringify(*os, p); }

  // AbslHashValue implements hashing for use with Abseil's hash-based
  // containers, like absl::flat_hash_{map,set}.
  template <typename H>
  friend H AbslHashValue(H h, const Pos& p) {
    return H::combine(std::move(h), p.x, p.y);
  }

  friend bool operator==(const Pos& lhs, const Pos& rhs) {
    return lhs.x == rhs.x && lhs.y == rhs.y;
  }
  friend bool operator!=(const Pos& lhs, const Pos& rhs) {
    return !(lhs == rhs);
  }

  // Arithmetic operators taken from https://stackoverflow.com/a/52377719.
  Pos& operator+=(const Pos& rhs) {
    x += rhs.x;
    y += rhs.y;
    return *this;
  }
  friend Pos operator+(Pos lhs, const Pos& rhs) {
    lhs += rhs;
    return lhs;
  }

  Pos& operator-=(const Pos& rhs) {
    x -= rhs.x;
    y -= rhs.y;
    return *this;
  }
  friend Pos operator-(Pos lhs, const Pos& rhs) {
    lhs -= rhs;
    return lhs;
  }
};
}  // namespace geometry
}  // namespace cc
}  // namespace adventofcode

#endif
