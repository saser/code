#include "adventofcode/cc/year2022/day15.h"

#include <string>

#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/string_view.h"
#include "adventofcode/cc/year2022/day15_shared.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day15 {

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return adventofcode::cc::year2022::day15_shared::Part1(input, 2000000);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return adventofcode::cc::year2022::day15_shared::Part2(input, 4000000);
}

}  // namespace day15
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
