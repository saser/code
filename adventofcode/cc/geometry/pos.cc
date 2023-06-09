#include "adventofcode/cc/geometry/pos.h"

#include <cmath>
#include <cstdint>

namespace adventofcode {
namespace cc {
namespace geometry {
int64_t Pos::Distance() const { return Distance(Pos{.x = 0, .y = 0}); }
int64_t Pos::Distance(const Pos& to) const {
  return std::abs(x - to.x) + std::abs(y - to.y);
}
}  // namespace geometry
}  // namespace cc
}  // namespace adventofcode
