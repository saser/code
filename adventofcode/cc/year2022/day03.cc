#include "adventofcode/cc/year2022/day03.h"

// The basic idea for part 1 is this:
// 1. Build up the set of item types seen in the first compartment.
// 2. Build up the set of item types seen in the second compartment.
// 3. Find the guaranteed exactly one item type in the intersection of these
//    sets.
// We know that the sets can only contain at most 52 elements (a-z + A-Z =
// 52). We can therefore use a 64-bit integer as a bitmask to implement a set
// with a capacity of 64: a "bitset". All characters get assigned a number,
// with 'a' = 0 and 'A' = 26 as baselines, which then becomes the bit number
// to set in the integer to represent the presence of this character.
//
// We build up two such bitsets, one for each compartment in the rucksack. We
// then take the bitwise AND (the binary & operator) of these bitsets and are
// left with an integer with exactly one bit set. We can then just calculate
// which bit number this is, and translate that back to an item type (using
// the same numbering as mentioned above) to know which item type is present
// in both compartments.
//
// Example with much smaller inputs:
//
//     line = "aHubO" + "tnHiq"
//
// The only character present in both halves is 'H'.
//
// The first compartment's bitset will look like:
//
// 0000000000000000000000010000001000000000000100000000000000000011
// ____________ZYXWVUTSRQPONMLKJIHGFEDCBAzyxwvutsrqponmlkjihgfedcba
//
// The second compartment's bitset will look like:
//
// 0000000000000000000000000000001000000000000010010010000100000000
// ____________ZYXWVUTSRQPONMLKJIHGFEDCBAzyxwvutsrqponmlkjihgfedcba
//
// (These representations are most significant bit first. The '_' bits are
// unused.)
//
// Taking the bitwise AND of these bitsets leaves us with:
//
//   0000000000000000000000010000001000000000000100000000000000000011 <first>
// & 0000000000000000000000000000001000000000000010010010000100000000 <second>
// = 0000000000000000000000000000001000000000000000000000000000000000 <result>
//   ____________ZYXWVUTSRQPONMLKJIHGFEDCBAzyxwvutsrqponmlkjihgfedcba
//                                 ^ only bit to be set in both bitsets
// From this we know that 'H' is the only item type present in both
// compartments. We can then translate it back to its priority ('H' = 34) and
// return that.
//
// ----------------------------------------------------------------------------
//
// For part 2:
// * we don't split one rucksack into two compartments, and
// * we have three rucksacks.
// However, the same idea can be used. To find the single item type shared by
// three rucksacks, we just take the intersection of three bitsets, rather than
// two.
//
// ----------------------------------------------------------------------------
//
// Because the problems are so similar, we can build a general solution and use
// two cases of it for the two parts of the problem. The general algorithm will
// be:
// 1. For each of N rucksacks, build a bitset.
// 2. Take the intersection of these bitsets.
// 3. Convert the result, guaranteed to only contain one elemnt, into a
// priority.
//
// For part 1, we split one input line into two compartments, and treat one
// compartment as one rucksack.
//
// For part 2, we split the input into groups of three lines, and treat each
// line as one rucksack.

#include <string>

#include "absl/status/statusor.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day03 {
namespace {

inline constexpr size_t kCharToBitnumSize = 'z' - 'A' + 1;

// charToBitnum implements a lookup table from a character to the bit number
// representing that character. It is intended to be used as follows:
//     bitnum = charToBitnum[c - 'A']
// where c is the character.
//
// Using a lookup table like this is done for performance. It also, fortunately,
// is quite space efficient: the ASCII table goes roughly like:
//     <a bunch of irrelevant characters>
//     A
//     ...
//     Z
//     <6 irrelevant characters>
//     a
//     ...
//     z
//     <a bunch of irrelevant characters>
// This means that creating a array that spans from 'A' to 'z' only contains 6
// irrelevant characters.
inline constexpr int charToBitnum[kCharToBitnumSize]{
    26,  // A
    27,  // B
    28,  // C
    29,  // D
    30,  // E
    31,  // F
    32,  // G
    33,  // H
    34,  // I
    35,  // J
    36,  // K
    37,  // L
    38,  // M
    39,  // N
    40,  // O
    41,  // P
    42,  // Q
    43,  // R
    44,  // S
    45,  // T
    46,  // U
    47,  // V
    48,  // W
    49,  // X
    50,  // Y
    51,  // Z
    0,   // [ <unused>
    0,   // backslash <unused>
    0,   // ] <unused>
    0,   // ^ <unused>
    0,   // _ <unused>
    0,   // ` <unused>
    0,   // a
    1,   // b
    2,   // c
    3,   // d
    4,   // e
    5,   // f
    6,   // g
    7,   // h
    8,   // i
    9,   // j
    10,  // k
    11,  // l
    12,  // m
    13,  // n
    14,  // o
    15,  // p
    16,  // q
    17,  // r
    18,  // s
    19,  // t
    20,  // u
    21,  // v
    22,  // w
    23,  // x
    24,  // y
    25,  // z
};

// bitset converts the rucksack contents into a bitset representing all item
// types that appear at least once.
//
// According to cppreference.com[0], an unsigned long long int is guaranteed to
// contain at least 64 bits.
//
// [0]: https://en.cppreference.com/w/cpp/language/types
unsigned long long int bitset(absl::string_view rucksack) {
  unsigned long long int b = 0;
  for (char c : rucksack) {
    b |= 1ull << charToBitnum[c - 'A'];
  }
  return b;
}

// priority assumes the bitset contains only a single bit, and converts that
// into the priority as specified in the problem description.
int priority(unsigned long long int bitset) {
  // Note: here is a much shorter implementation of this function:
  //     int bitnum;
  //     for (bitnum = 0; bitnum < 52; bitnum++) {
  //       if ((bitset & (1ull << bitnum)) != 0) {
  //         break;
  //       }
  //     }
  //     return bitnum + 1;
  // Using a switch statement is slightly faster, at least on my system.
  int bitnum;
  switch (bitset) {
    case 1ull << 0:
      bitnum = 0;
      break;
    case 1ull << 1:
      bitnum = 1;
      break;
    case 1ull << 2:
      bitnum = 2;
      break;
    case 1ull << 3:
      bitnum = 3;
      break;
    case 1ull << 4:
      bitnum = 4;
      break;
    case 1ull << 5:
      bitnum = 5;
      break;
    case 1ull << 6:
      bitnum = 6;
      break;
    case 1ull << 7:
      bitnum = 7;
      break;
    case 1ull << 8:
      bitnum = 8;
      break;
    case 1ull << 9:
      bitnum = 9;
      break;
    case 1ull << 10:
      bitnum = 10;
      break;
    case 1ull << 11:
      bitnum = 11;
      break;
    case 1ull << 12:
      bitnum = 12;
      break;
    case 1ull << 13:
      bitnum = 13;
      break;
    case 1ull << 14:
      bitnum = 14;
      break;
    case 1ull << 15:
      bitnum = 15;
      break;
    case 1ull << 16:
      bitnum = 16;
      break;
    case 1ull << 17:
      bitnum = 17;
      break;
    case 1ull << 18:
      bitnum = 18;
      break;
    case 1ull << 19:
      bitnum = 19;
      break;
    case 1ull << 20:
      bitnum = 20;
      break;
    case 1ull << 21:
      bitnum = 21;
      break;
    case 1ull << 22:
      bitnum = 22;
      break;
    case 1ull << 23:
      bitnum = 23;
      break;
    case 1ull << 24:
      bitnum = 24;
      break;
    case 1ull << 25:
      bitnum = 25;
      break;
    case 1ull << 26:
      bitnum = 26;
      break;
    case 1ull << 27:
      bitnum = 27;
      break;
    case 1ull << 28:
      bitnum = 28;
      break;
    case 1ull << 29:
      bitnum = 29;
      break;
    case 1ull << 30:
      bitnum = 30;
      break;
    case 1ull << 31:
      bitnum = 31;
      break;
    case 1ull << 32:
      bitnum = 32;
      break;
    case 1ull << 33:
      bitnum = 33;
      break;
    case 1ull << 34:
      bitnum = 34;
      break;
    case 1ull << 35:
      bitnum = 35;
      break;
    case 1ull << 36:
      bitnum = 36;
      break;
    case 1ull << 37:
      bitnum = 37;
      break;
    case 1ull << 38:
      bitnum = 38;
      break;
    case 1ull << 39:
      bitnum = 39;
      break;
    case 1ull << 40:
      bitnum = 40;
      break;
    case 1ull << 41:
      bitnum = 41;
      break;
    case 1ull << 42:
      bitnum = 42;
      break;
    case 1ull << 43:
      bitnum = 43;
      break;
    case 1ull << 44:
      bitnum = 44;
      break;
    case 1ull << 45:
      bitnum = 45;
      break;
    case 1ull << 46:
      bitnum = 46;
      break;
    case 1ull << 47:
      bitnum = 47;
      break;
    case 1ull << 48:
      bitnum = 48;
      break;
    case 1ull << 49:
      bitnum = 49;
      break;
    case 1ull << 50:
      bitnum = 50;
      break;
    case 1ull << 51:
      bitnum = 51;
      break;
  }
  // Bit numbers start at 0, priorities start at 1.
  return bitnum + 1;
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  int sum = 0;
  for (absl::string_view line : absl::StrSplit(input, '\n')) {
    if (line == "") {
      // End of input, if the input contains a trailing newline.
      break;
    }
    unsigned long long int intersection = ~0ull;  // set all bits
    const size_t half = line.length() / 2;
    intersection &= bitset(line.substr(0, half));
    intersection &= bitset(line.substr(half, half));
    sum += priority(intersection);
  }
  return std::to_string(sum);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  int sum = 0;

  // intersection contains the accumulated intersection so far.
  unsigned long long int intersection = ~0ull;  // set all bits
  // n is how many rucksacks we have intersected so far.
  int n = 0;
  for (absl::string_view line : absl::StrSplit(input, '\n')) {
    if (line == "") {
      // End of input, if the input contains a trailing newline.
      break;
    }
    intersection &= bitset(line);
    n++;

    // The elves are split into groups of 3, so when we have taken the
    // intersection of 3 rucksacks, the intersection should contain the only
    // item type present in all 3 rucksacks. Convert that to priority and then
    // reset the accumulators.
    if (n == 3) {
      sum += priority(intersection);
      intersection = ~0ull;  // set all bits
      n = 0;
      continue;
    }
  }
  return std::to_string(sum);
}
}  // namespace day03
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
