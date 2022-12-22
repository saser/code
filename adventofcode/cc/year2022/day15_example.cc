#include "adventofcode/cc/year2022/day15_example.h"

#include <string>

#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/string_view.h"
#include "adventofcode/cc/year2022/day15_shared.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day15_example {

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return adventofcode::cc::year2022::day15_shared::Part1(input, 10);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return adventofcode::cc::year2022::day15_shared::Part2(input, 20);
}

}  // namespace day15_example
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
