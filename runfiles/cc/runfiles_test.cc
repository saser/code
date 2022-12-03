#include "runfiles/cc/runfiles.h"

#include "absl/status/status.h"
#include "gtest/gtest.h"

TEST(Runfiles, PathForTest) {
  const auto got =
      runfiles::cc::runfiles::PathForTest("code/runfiles/test.txt");
  ASSERT_TRUE(got.ok()) << "Unexpected error: " << got.status();
  EXPECT_NE(*got, "");
}

TEST(Runfiles, ReadForTest) {
  const auto got =
      runfiles::cc::runfiles::ReadForTest("code/runfiles/test.txt");
  const auto want = "This is an example file to be used in tests.\n";
  ASSERT_TRUE(got.ok()) << "Unexpected error: " << got.status();
  EXPECT_EQ(*got, want);
}

TEST(Runfiles, PathForTestNotFound) {
  const auto got =
      runfiles::cc::runfiles::PathForTest("code/runfiles/does_not_exist.txt");
  ASSERT_EQ(got.status().code(), absl::StatusCode::kNotFound);
}

TEST(Runfiles, ReadForTestNotFound) {
  const auto got =
      runfiles::cc::runfiles::ReadForTest("code/runfiles/does_not_exist.txt");
  ASSERT_EQ(got.status().code(), absl::StatusCode::kNotFound);
}
