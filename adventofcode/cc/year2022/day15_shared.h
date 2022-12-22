#ifndef ADVENTOFCODE_CC_YEAR2022_DAY15_SHARED_H_
#define ADVENTOFCODE_CC_YEAR2022_DAY15_SHARED_H_

#include <string>

#include "absl/status/statusor.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day15_shared {
absl::StatusOr<std::string> Part1(absl::string_view input, int target_y);
absl::StatusOr<std::string> Part2(absl::string_view input, int xy_max);
}  // namespace day15_shared
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode

#endif
