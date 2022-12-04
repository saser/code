#include "adventofcode/cc/year2022/day01.h"

#include "gtest/gtest.h"
#include "runfiles/cc/runfiles.h"

TEST(Day01, Part1Example) {
  const auto input = runfiles::cc::runfiles::ReadForTest(
      "code/adventofcode/data/year2022/day01.example.in");
  ASSERT_TRUE(input.ok()) << "Unexpected error: " << input.status();
  const auto got = adventofcode::cc::year2022::day01::Part1(*input);
  const auto want = "24000";
  ASSERT_TRUE(got.ok()) << "Unexpected error: " << got.status();
  EXPECT_EQ(*got, want);
}

TEST(Day01, Part1Real) {
  const auto input = runfiles::cc::runfiles::ReadForTest(
      "code/adventofcode/data/year2022/day01.real.in");
  ASSERT_TRUE(input.ok()) << "Unexpected error: " << input.status();
  const auto got = adventofcode::cc::year2022::day01::Part1(*input);
  const auto want = "68787";
  ASSERT_TRUE(got.ok()) << "Unexpected error: " << got.status();
  EXPECT_EQ(*got, want);
}

TEST(Day01, Part2Example) {
  const auto input = runfiles::cc::runfiles::ReadForTest(
      "code/adventofcode/data/year2022/day01.example.in");
  ASSERT_TRUE(input.ok()) << "Unexpected error: " << input.status();
  const auto got = adventofcode::cc::year2022::day01::Part2(*input);
  const auto want = "45000";
  ASSERT_TRUE(got.ok()) << "Unexpected error: " << got.status();
  EXPECT_EQ(*got, want);
}

TEST(Day01, Part2Real) {
  const auto input = runfiles::cc::runfiles::ReadForTest(
      "code/adventofcode/data/year2022/day01.real.in");
  ASSERT_TRUE(input.ok()) << "Unexpected error: " << input.status();
  const auto got = adventofcode::cc::year2022::day01::Part2(*input);
  const auto want = "198041";
  ASSERT_TRUE(got.ok()) << "Unexpected error: " << got.status();
  EXPECT_EQ(*got, want);
}
