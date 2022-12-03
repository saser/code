#ifndef RUNFILES_CC_RUNFILES_H_
#define RUNFILES_CC_RUNFILES_H_

#include "absl/status/statusor.h"
#include "absl/strings/string_view.h"

namespace runfiles {
namespace cc {
namespace runfiles {
// Path returns the absolute path for the given runfiles, if it exists. If it
// doesn't exist, the status contains an error. The runfile argument _must_
// contain the name of the repository of the runfile, even if it is in the local
// repository.
//
// Use this in `cc_library` and  `cc_binary` targets, and use PathForTest in
// `cc_test` targets.
absl::StatusOr<std::string> Path(absl::string_view runfile,
                                 absl::string_view argv0 = "");

// Read is like Path, but returns the contents of the file rather than the
// absolute path to it.
//
// Use this in `cc_library` and  `cc_binary` targets, and use ReadForTest in
// `cc_test` targets.
absl::StatusOr<std::string> Read(absl::string_view runfile,
                                 absl::string_view argv0 = "");

// PathForTest returns the absolute path for the given runfile, if it exists. If
// it doesn't exist, the status contains an error. The runfile argument _must_
// contain the name of the repository of the runfile, even if it is in the local
// repository.
//
// Use this in `cc_test` targets, and use Path in `cc_library` and `cc_binary`
// targets.
absl::StatusOr<std::string> PathForTest(absl::string_view runfile);

// ReadForTest is like PathForTest, but returns the contents of the file rather
// than the path to it.
//
// Use this in `cc_test` targets, and use Path in `cc_library` and `cc_binary`
// targets.
absl::StatusOr<std::string> ReadForTest(absl::string_view runfile);
}  // namespace runfiles
}  // namespace cc
}  // namespace runfiles

#endif
