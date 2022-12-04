#include "adventofcode/cc/year2022/day02.h"

#include "gtest/gtest.h"
#include "runfiles/cc/runfiles.h"

TEST(Day02, Part1Example) {
  const auto input = runfiles::cc::runfiles::ReadForTest(
      "code/adventofcode/data/year2022/day02.example.in");
  ASSERT_TRUE(input.ok()) << "Unexpected error: " << input.status();
  const auto got = adventofcode::cc::year2022::day02::Part1(*input);
  const auto want = "15";
  ASSERT_TRUE(got.ok()) << "Unexpected error: " << got.status();
  EXPECT_EQ(*got, want);
}

TEST(Day02, Part1Real) {
  const auto input = runfiles::cc::runfiles::ReadForTest(
      "code/adventofcode/data/year2022/day02.real.in");
  ASSERT_TRUE(input.ok()) << "Unexpected error: " << input.status();
  const auto got = adventofcode::cc::year2022::day02::Part1(*input);
  const auto want = "13924";
  ASSERT_TRUE(got.ok()) << "Unexpected error: " << got.status();
  EXPECT_EQ(*got, want);
}

TEST(Day02, Part2Example) {
  const auto input = runfiles::cc::runfiles::ReadForTest(
      "code/adventofcode/data/year2022/day02.example.in");
  ASSERT_TRUE(input.ok()) << "Unexpected error: " << input.status();
  const auto got = adventofcode::cc::year2022::day02::Part2(*input);
  const auto want = "12";
  ASSERT_TRUE(got.ok()) << "Unexpected error: " << got.status();
  EXPECT_EQ(*got, want);
}

TEST(Day02, Part2Real) {
  const auto input = runfiles::cc::runfiles::ReadForTest(
      "code/adventofcode/data/year2022/day02.real.in");
  ASSERT_TRUE(input.ok()) << "Unexpected error: " << input.status();
  const auto got = adventofcode::cc::year2022::day02::Part2(*input);
  const auto want = "13448";
  ASSERT_TRUE(got.ok()) << "Unexpected error: " << got.status();
  EXPECT_EQ(*got, want);
}
