#include "adventofcode/cc/year2022/day01.h"

#include <algorithm>
#include <string>
#include <vector>

#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day01 {
namespace {
absl::StatusOr<std::vector<int>> calories(absl::string_view input) {
  std::vector<int> sums;
  // Assume the input contains a trailing newline, and therefore the last
  // element is an empty line. That means that each group of calories is
  // _terminated_ by a blank line, not _separated_. So each time we encounter a
  // blank line, we can end the sum and reset it to 0;
  std::vector<absl::string_view> lines = absl::StrSplit(input, '\n');
  int sum = 0;
  for (auto line : lines) {
    if (line == "") {
      sums.push_back(sum);
      sum = 0;
      continue;
    }
    int v;
    if (!absl::SimpleAtoi(line, &v)) {
      return absl::InvalidArgumentError(
          "invalid line couldn't be parsed as an integer: " +
          std::string(line));
    }
    sum += v;
  }
  return sums;
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  const auto sums = calories(input);
  if (!sums.ok()) {
    return absl::Status(sums.status());
  }
  const auto max = std::max_element(sums->begin(), sums->end());
  return std::to_string(*max);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  auto sums = calories(input);
  if (!sums.ok()) {
    return absl::Status(sums.status());
  }
  // std::sort sorts in ascending order; use reverse iterator to get descending
  // order.
  std::sort(sums->rbegin(), sums->rend());
  const auto sum = (*sums)[0] + (*sums)[1] + (*sums)[2];
  return std::to_string(sum);
}
}  // namespace day01
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
