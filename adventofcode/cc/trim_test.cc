#include "adventofcode/cc/trim.h"

#include <unordered_map>

#include "gtest/gtest.h"

TEST(TrimTest, TrimSpace) {
  EXPECT_EQ(adventofcode::cc::trim::TrimSpace(""), "");
  EXPECT_EQ(adventofcode::cc::trim::TrimSpace("hello"), "hello");
  EXPECT_EQ(adventofcode::cc::trim::TrimSpace(" hello"), "hello");
  EXPECT_EQ(adventofcode::cc::trim::TrimSpace("hello "), "hello");
  EXPECT_EQ(adventofcode::cc::trim::TrimSpace(" hello "), "hello");
  EXPECT_EQ(adventofcode::cc::trim::TrimSpace("hello\n"), "hello");
  EXPECT_EQ(adventofcode::cc::trim::TrimSpace("\nhello\n"), "hello");
  EXPECT_EQ(adventofcode::cc::trim::TrimSpace("   \nhello  \n   \n   \n"),
            "hello");
}
