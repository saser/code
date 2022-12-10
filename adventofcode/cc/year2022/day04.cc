#include "adventofcode/cc/year2022/day04.h"

#include <string>
#include <vector>

#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_format.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day04 {
namespace {
struct Span {
  int start;
  int end;

  std::string String() const { return absl::StrFormat("%d-%d", start, end); }
};

bool contains(const Span& s1, const Span& s2) {
  if (s1.start == s2.start || s1.end == s2.end) {
    return true;
  }
  if (s1.start < s2.start) {
    return s2.end <= s1.end;
  }
  // Here we know that s2.start < s1.start.
  return s1.end <= s2.end;
}

bool overlaps(const Span& s1, const Span& s2) {
  if (s1.start < s2.start) {
    return s2.start <= s1.end;
  }
  if (s2.start < s1.start) {
    return s1.start <= s2.end;
  }
  // Here we know that s1.start == s2.start, which means they fully overlap,
  // which also means that they partially overlap.
  return true;
}

struct Assignment {
  Span first;
  Span second;

  std::string String() const {
    return absl::StrFormat("%s,%s", first.String(), second.String());
  }
};

absl::StatusOr<Assignment> parseLine(absl::string_view line) {
  // The easiest solution here is to use absl::StrSplit together with
  // absl::ByAnyChar to split on either '-' or ',' but that performs worse than
  // just seeking in the string for the expected delimiters.
  Assignment a;
  // start and end will be used to take substrings of line.
  size_t start, end;
  // sub will be used to store the substrings.
  absl::string_view sub;
  // ok will hold the result of trying to parse the substrings as integers. It
  // will only be checked in debug builds.

  // First substring: start of first span.
  start = 0;
  end = line.find('-', start);
  sub = line.substr(start, end - start);
  if (!absl::SimpleAtoi(sub, &a.first.start)) {
    return absl::InvalidArgumentError("invalid part: " + std::string(sub));
  }

  // Second substring: end of first span.
  start = end + 1;  // skip over '-'
  end = line.find(',', start);
  sub = line.substr(start, end - start);
  if (!absl::SimpleAtoi(sub, &a.first.end)) {
    return absl::InvalidArgumentError("invalid part: " + std::string(sub));
  }

  // Third substring: start of second span.
  start = end + 1;  // skip over ','
  end = line.find('-', start);
  sub = line.substr(start, end - start);
  if (!absl::SimpleAtoi(sub, &a.second.start)) {
    return absl::InvalidArgumentError("invalid part: " + std::string(sub));
  }

  // Fourth substring: end of second span.
  start = end + 1;  // skip over '-'
  end = line.find(',', start);
  sub = line.substr(start);  // => to end of line
  if (!absl::SimpleAtoi(sub, &a.second.end)) {
    return absl::InvalidArgumentError("invalid part: " + std::string(sub));
  }

  return a;
}

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  int count = 0;
  for (absl::string_view line : absl::StrSplit(input, '\n')) {
    if (line == "") {
      // End of input due to trailing newline.
      break;
    }
    absl::StatusOr<Assignment> a = parseLine(line);
    if (!a.ok()) {
      return absl::Status(a.status());
    }
    if (part1 ? contains(a->first, a->second) : overlaps(a->first, a->second)) {
      count++;
    }
  }
  return std::to_string(count);
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}
}  // namespace day04
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
