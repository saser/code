#include "adventofcode/cc/trim.h"

#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace trim {
absl::string_view TrimSpace(absl::string_view s) {
  static constexpr absl::string_view whitespace = " \n";
  auto start = s.find_first_not_of(whitespace);
  if (start == absl::string_view::npos) {
    start = 0;
  }
  auto end = s.find_last_not_of(whitespace);
  if (end == absl::string_view::npos) {
    end = s.length();
  }
  return s.substr(start, end - start + 1);
}
}  // namespace trim
}  // namespace cc
}  // namespace adventofcode
