#ifndef ADVENTOFCODE_CC_YEAR2022_DAY06_H_
#define ADVENTOFCODE_CC_YEAR2022_DAY06_H_

#include <string>

#include "absl/status/statusor.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day06 {
absl::StatusOr<std::string> Part1(absl::string_view input);
absl::StatusOr<std::string> Part2(absl::string_view input);
}  // namespace day06
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode

#endif
