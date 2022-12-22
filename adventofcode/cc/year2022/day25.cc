#include "adventofcode/cc/year2022/day25.h"

#include <cstdint>
#include <sstream>
#include <string>
#include <vector>

#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day25 {
namespace {

int64_t snafuToDecimal(absl::string_view snafu) {
  int64_t sum = 0;
  int64_t factor = 1;
  for (auto it = snafu.crbegin(); it != snafu.crend(); it++) {
    char c = *it;
    int64_t x;
    switch (c) {
      case '2':
      case '1':
      case '0':
        x = c - '0';
        break;
      case '-':
        x = -1;
        break;
      case '=':
        x = -2;
        break;
    }
    sum += x * factor;
    factor *= 5;
  }
  return sum;
}

std::string decimalToSnafu(int64_t input) {
  // The implementation of this function is roughly:
  // 1. Represent the number in base-5 (digits: 0 through 4).
  // 2. Convert that into base-snafu (digits: -2 through 2).
  // 3. Convert _that_ into a string.

  // First represent as regular base-5. Note that digits is ordered from least
  // to most significant digit. This differs from how numbers are usually
  // represented, which is most to least significant digit. We will account for
  // this later when we convert from integers into a string.
  std::vector<int64_t> digits;
  for (int64_t n = input; n != 0; n /= 5) {
    digits.push_back(n % 5);
  }
  // We're going to replace base-5 with base-snafu in-place. Base-snafu might
  // require one extra digit (if the most significant digit turns out to be 3,
  // 4, or 5), so insert an extra leading 0. After we have converted from base-5
  // to base-snafu, if the leading digit is still a 0 we drop it again.
  digits.push_back(0);
  for (size_t i = 0; i < digits.size(); i++) {
    int64_t& d = digits[i];
    if (d >= 3) {
      d -= 5;
      digits[i + 1]++;
    }
  }
  if (digits.back() == 0) {
    digits.pop_back();
  }

  // Iterate through digits in reverse order -- the string representation has
  // the most significant digit first, while digits has the least significant
  // digit first.
  std::stringstream s;
  for (auto it = digits.crbegin(); it != digits.crend(); it++) {
    int64_t digit = *it;
    char c;
    switch (digit) {
      case -2:
        c = '=';
        break;
      case -1:
        c = '-';
        break;
      case 0:
      case 1:
      case 2:
        c = '0' + digit;
        break;
    }
    s << c;
  }
  return s.str();
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  int64_t sum = 0;
  for (absl::string_view snafu :
       absl::StrSplit(input, '\n', absl::SkipEmpty())) {
    sum += snafuToDecimal(snafu);
  }
  return decimalToSnafu(sum);
}
}  // namespace day25
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
