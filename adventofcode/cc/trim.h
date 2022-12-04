#ifndef ADVENTOFCODE_CC_TRIM_H_
#define ADVENTOFCODE_CC_TRIM_H_

#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace trim {
// TrimSpace removes leading and trailing space characters (' ') and newline
// characters ('\n') and returns a new absl::string_view backed by the same data
// as s.
absl::string_view TrimSpace(absl::string_view s);
}  // namespace trim
}  // namespace cc
}  // namespace adventofcode
#endif
