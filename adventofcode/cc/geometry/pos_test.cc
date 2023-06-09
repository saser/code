#include "adventofcode/cc/geometry/pos.h"

#include "absl/hash/hash_testing.h"
#include "absl/strings/str_format.h"
#include "gtest/gtest.h"

namespace adventofcode {
namespace cc {
namespace geometry {
TEST(PosTest, Distance) {
  EXPECT_EQ((Pos{.x = 0, .y = 0}).Distance(), 0);
  EXPECT_EQ((Pos{.x = +1, .y = 0}).Distance(), 1);
  EXPECT_EQ((Pos{.x = -1, .y = 0}).Distance(), 1);
  EXPECT_EQ((Pos{.x = +1, .y = +1}).Distance(), 2);
  EXPECT_EQ((Pos{.x = -1, .y = +1}).Distance(), 2);
  EXPECT_EQ((Pos{.x = +1, .y = -1}).Distance(), 2);
  EXPECT_EQ((Pos{.x = -1, .y = -1}).Distance(), 2);
}

TEST(PosTest, DistanceTo) {
  EXPECT_EQ((Pos{.x = 0, .y = 0}).Distance(Pos{.x = 0, .y = 0}), 0);
  EXPECT_EQ((Pos{.x = +1, .y = 0}).Distance(Pos{.x = +1, .y = 0}), 0);
  EXPECT_EQ((Pos{.x = -1, .y = 0}).Distance(Pos{.x = +1, .y = 0}), 2);
  EXPECT_EQ((Pos{.x = -1, .y = -1}).Distance(Pos{.x = +1, .y = +1}), 4);
}

TEST(PosTest, AbslStringify) {
  // Using absl::StrFormat here is my lazy version of calling the AbslStringify
  // function.
  EXPECT_EQ(absl::StrFormat("%v", Pos{.x = +1, .y = +1}), "(1,1)");
  EXPECT_EQ(absl::StrFormat("%v", Pos{.x = +1, .y = -1}), "(1,-1)");
  EXPECT_EQ(absl::StrFormat("%v", Pos{.x = -1, .y = +1}), "(-1,1)");
  EXPECT_EQ(absl::StrFormat("%v", Pos{.x = -1, .y = -1}), "(-1,-1)");
}

TEST(PosTest, AbslHashValue) {
  EXPECT_TRUE(absl::VerifyTypeImplementsAbslHashCorrectly({
      Pos{.x = 0, .y = 0},
      Pos{.x = +1, .y = +1},
      Pos{.x = +1, .y = -1},
      Pos{.x = -1, .y = +1},
      Pos{.x = -1, .y = -1},
      Pos{.x = -123, .y = +456},
  }));
}

TEST(PosTest, Addition) {
  Pos a{.x = 1, .y = 2};
  Pos b{.x = 10, .y = 10};
  Pos want{.x = 11, .y = 12};
  EXPECT_EQ(a + b, want);
  EXPECT_EQ(b + a, want);
  Pos a2 = a;
  a2 += b;
  EXPECT_EQ(a2, want);
  Pos b2 = b;
  b2 += a;
  EXPECT_EQ(b2, want);
}

TEST(PosTest, Subtraction) {
  Pos a{.x = 1, .y = 2};
  Pos b{.x = 10, .y = 10};
  Pos amb{.x = -9, .y = -8};
  Pos bma{.x = 9, .y = 8};
  EXPECT_EQ(a - b, amb);
  EXPECT_EQ(b - a, bma);
  Pos a2 = a;
  a2 -= b;
  EXPECT_EQ(a2, amb);
  Pos b2 = b;
  b2 -= a;
  EXPECT_EQ(b2, bma);
}
}  // namespace geometry
}  // namespace cc
}  // namespace adventofcode
