#include "adventofcode/cc/year2022/day11.h"

#include <algorithm>
#include <cstdint>
#include <sstream>
#include <string>
#include <vector>

#include "absl/log/check.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_format.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"
#include "adventofcode/cc/trim.h"
#include "re2/re2.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day11 {
namespace {
struct Monkey {
  size_t n;                     // Monkey number.
  std::vector<uint64_t> items;  // currently held items.

  // These fields encode the operation.
  uint64_t lhs;  // 0 == old
  bool op;       // false = +, true = *
  uint64_t rhs;  // 0 == old

  uint64_t mod;     // The number to test divisibility with.
  size_t if_true;   // Where to throw if the item is divisible by mod.
  size_t if_false;  // Where to throw if the item is not divisible by mod.

  // Parse returns the Monkey parsed from a string looking like:
  //   Monkey N:
  //     Starting items: X, Y, Z, ...
  //     Operation: new = lhs */+ rhs
  //     Test: divisible by M
  //       If true: throw to monkey A
  //       If true: throw to monkey B
  //
  // `lhs` and `rhs` are either the "old" or a positive integer.
  static Monkey Parse(absl::string_view fragment) {
    size_t n;
    std::string items_str;
    std::string lhs_str;
    std::string op_str;
    std::string rhs_str;
    uint64_t mod;
    size_t if_true;
    size_t if_false;
    static RE2 monkey_regex(R"((?m)Monkey (?P<n>\d+):
  Starting items: (?P<items>[0-9, ]+)
  Operation: new = (?P<lhs>old|\d+) (?P<op>[+*]) (?P<rhs>old|\d+)
  Test: divisible by (?P<mod>\d+)
    If true: throw to monkey (?P<if_true>\d+)
    If false: throw to monkey (?P<if_false>\d+))");
    CHECK(RE2::FullMatch(fragment, monkey_regex, &n, &items_str, &lhs_str,
                         &op_str, &rhs_str, &mod, &if_true, &if_false));

    std::vector<uint64_t> items;
    for (absl::string_view part : absl::StrSplit(items_str, ", ")) {
      uint64_t item;
      CHECK(absl::SimpleAtoi(part, &item));
      items.push_back(item);
    }

    uint64_t lhs;
    if (lhs_str == "old") {
      lhs = 0;
    } else {
      CHECK(absl::SimpleAtoi(lhs_str, &lhs));
    }
    bool op = op_str == "*";
    uint64_t rhs;
    if (rhs_str == "old") {
      rhs = 0;
    } else {
      CHECK(absl::SimpleAtoi(rhs_str, &rhs));
    }

    Monkey m;
    m.n = n;
    m.items = items;
    m.lhs = lhs;
    m.op = op;
    m.rhs = rhs;
    m.mod = mod;
    m.if_false = if_false;
    m.if_true = if_true;
    return m;
  }

  uint64_t Apply(uint64_t x) const {
    uint64_t a = (lhs == 0) ? x : lhs;
    uint64_t b = (rhs == 0) ? x : rhs;
    return op ? a * b : a + b;
  }
};

class MonkeySet {
 public:
  MonkeySet() = delete;
  MonkeySet(std::vector<Monkey> monkeys, bool super_worried)
      : monkeys_(monkeys),
        inspections_(monkeys_.size()),
        super_worried_(super_worried),
        mod_product_(0) {
    mod_product_ = 1;
    for (const Monkey& m : monkeys_) {
      mod_product_ *= m.mod;
    }
  }

  std::vector<uint64_t> Inspections() const { return inspections_; }

  void DoRound() {
    for (size_t n = 0; n < monkeys_.size(); n++) {
      Monkey& m = monkeys_[n];
      for (uint64_t item : m.items) {
        item = m.Apply(item);
        item = super_worried_ ? item % mod_product_ : item / 3;
        size_t next = (item % m.mod == 0) ? m.if_true : m.if_false;
        monkeys_[next].items.push_back(item);
      }
      inspections_[n] += m.items.size();
      m.items.clear();
    }
  }

  std::string DebugString() const {
    std::stringstream buf;
    for (size_t n = 0; n < monkeys_.size(); n++) {
      buf << absl::StreamFormat("Monkey %d: %d inspections, items: ", n,
                                inspections_[n]);
      for (const uint64_t& item : monkeys_[n].items) {
        buf << absl::StreamFormat("%d, ", item);
      }
      buf << std::endl;
    }
    return buf.str();
  }

 private:
  std::vector<Monkey> monkeys_;        // The monkeys.
  std::vector<uint64_t> inspections_;  // Monkey number -> # of inspections

  bool super_worried_;    // Whether we are super worried (part 2).
  uint64_t mod_product_;  // The product of all mod values.
};

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  std::vector<Monkey> monkeys;
  for (absl::string_view fragment :
       absl::StrSplit(adventofcode::cc::trim::TrimSpace(input), "\n\n")) {
    monkeys.push_back(Monkey::Parse(fragment));
  }

  MonkeySet ms(monkeys, !part1);
  int rounds = part1 ? 20 : 10000;
  for (int i = 0; i < rounds; i++) {
    ms.DoRound();
  }
  std::vector<uint64_t> inspections = ms.Inspections();
  std::sort(inspections.rbegin(), inspections.rend());
  return std::to_string(inspections[0] * inspections[1]);
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}

}  // namespace day11
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
