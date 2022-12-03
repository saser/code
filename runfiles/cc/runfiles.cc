#include "runfiles/cc/runfiles.h"

#include <filesystem>
#include <fstream>
#include <sstream>
#include <string>

#include "absl/status/statusor.h"
#include "absl/strings/string_view.h"
#include "tools/cpp/runfiles/runfiles.h"

namespace runfiles {
namespace cc {
namespace runfiles {
namespace {
absl::StatusOr<std::string> runfilePath(
    absl::string_view runfile,
    bazel::tools::cpp::runfiles::Runfiles* runfiles) {
  const auto path = runfiles->Rlocation(std::string(runfile));
  if (path == "") {
    return absl::NotFoundError("runfiles: couldn't find runfile " +
                               std::string(runfile));
  }
  // runfiles->Rlocation may return a path even for a file that doesn't exist,
  // for some reason. The documentation states that the caller should check for
  // the existence of the file.
  if (!std::filesystem::exists(path)) {
    return absl::NotFoundError("runfiles: couldn't find runfile " +
                               std::string(runfile) + " (path: " + path + ")");
  }
  return path;
}

absl::StatusOr<std::string> readRunfile(
    absl::string_view runfile,
    bazel::tools::cpp::runfiles::Runfiles* runfiles) {
  const auto path = runfilePath(runfile, runfiles);
  if (!path.ok()) {
    return path;
  }
  std::ifstream f(*path);
  std::stringstream buf;
  buf << f.rdbuf();
  if (f.fail()) {
    return absl::UnknownError("runfiles: couldn't read runfile " +
                              std::string(runfile));
  }
  return buf.str();
}
}  // namespace

absl::StatusOr<std::string> Path(absl::string_view runfile,
                                 absl::string_view argv0) {
  std::string error;
  std::unique_ptr<bazel::tools::cpp::runfiles::Runfiles> runfiles(
      bazel::tools::cpp::runfiles::Runfiles::Create(std::string(argv0),
                                                    &error));
  if (runfiles == nullptr) {
    return absl::UnknownError("runfiles: " + error);
  }
  return runfilePath(runfile, runfiles.get());
}

absl::StatusOr<std::string> Read(absl::string_view runfile,
                                 absl::string_view argv0) {
  std::string error;
  std::unique_ptr<bazel::tools::cpp::runfiles::Runfiles> runfiles(
      bazel::tools::cpp::runfiles::Runfiles::Create(std::string(argv0),
                                                    &error));
  if (runfiles == nullptr) {
    return absl::UnknownError("runfiles: " + error);
  }
  return readRunfile(runfile, runfiles.get());
}

absl::StatusOr<std::string> PathForTest(absl::string_view runfile) {
  std::string error;
  std::unique_ptr<bazel::tools::cpp::runfiles::Runfiles> runfiles(
      bazel::tools::cpp::runfiles::Runfiles::CreateForTest(&error));
  if (runfiles == nullptr) {
    return absl::InternalError("runfiles: " + error);
  }
  return runfilePath(runfile, runfiles.get());
}

absl::StatusOr<std::string> ReadForTest(absl::string_view runfile) {
  std::string error;
  std::unique_ptr<bazel::tools::cpp::runfiles::Runfiles> runfiles(
      bazel::tools::cpp::runfiles::Runfiles::CreateForTest(&error));
  if (runfiles == nullptr) {
    return absl::UnknownError("runfiles: " + error);
  }
  return readRunfile(runfile, runfiles.get());
}
}  // namespace runfiles
}  // namespace cc
}  // namespace runfiles
