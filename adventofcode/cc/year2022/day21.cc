#include "adventofcode/cc/year2022/day21.h"

#include <cstdint>
#include <string>
#include <variant>
#include <vector>

#include "absl/container/flat_hash_map.h"
#include "absl/log/check.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day21 {
namespace {

// Monkey represents a single monkey.
class Monkey {
 public:
  // Const represents a constant integer.
  struct Constant {
    int64_t v;
  };

  // Binary represents a binary arithmetic operation.
  struct Binary {
    std::string left;
    char op;
    std::string right;
  };

  Monkey() = delete;
  Monkey(absl::string_view name, int64_t v)
      : name_(name), expr_(Constant{.v = v}) {}
  Monkey(absl::string_view name, absl::string_view left, char op,
         absl::string_view right)
      : name_(name),
        expr_(Binary{
            .left = std::string(left), .op = op, .right = std::string(right)}) {
  }

  std::variant<Constant, Binary> Expr() const { return expr_; }

  Binary MustBinary() const { return std::get<Binary>(expr_); }

 private:
  std::string name_;

  // expr_ is either a constant integer or a binary arithmetic expression.
  std::variant<Constant, Binary> expr_;
};

absl::flat_hash_map<std::string, Monkey> Parse(absl::string_view input) {
  std::vector<absl::string_view> lines =
      absl::StrSplit(input, '\n', absl::SkipEmpty());
  // By preallocating the right number of elements we appear to save a bunch of
  // time.
  absl::flat_hash_map<std::string, Monkey> monkeys(lines.size());
  for (absl::string_view line : lines) {
    // This split will result in one of the two following forms:
    // 1. ["asbo", "12"] in case of a constant.
    // 2. ["bioa", "bhja", "+", "buyi"] in case of an operation.
    std::vector<absl::string_view> parts =
        absl::StrSplit(line, absl::ByAnyChar(": "), absl::SkipEmpty());
    CHECK(parts.size() == 2 || parts.size() == 4) << line;
    absl::string_view name = parts[0];
    switch (parts.size()) {
      case 2:
        int64_t v;
        CHECK(absl::SimpleAtoi(parts[1], &v));
        monkeys.insert_or_assign(name, Monkey(name, v));
        break;
      case 4:
        monkeys.insert_or_assign(
            name, Monkey(name, /*left=*/parts[1], /*op=*/parts[2][0],
                         /*right=*/parts[3]));
        break;
    }
  }
  return monkeys;
}

// Value represents what a Monkey evaluates to.
struct Value {
  int64_t value;
  bool has_humn;
};

void Evaluate(const absl::flat_hash_map<std::string, Monkey>& monkeys,
              absl::flat_hash_map<std::string, Value>& values,
              absl::string_view node) {
  const Monkey& m = monkeys.at(node);
  Value v;
  if (std::variant<Monkey::Constant, Monkey::Binary> expr = m.Expr();
      std::holds_alternative<Monkey::Constant>(expr)) {
    Monkey::Constant c = std::get<Monkey::Constant>(expr);
    v.value = c.v;
    v.has_humn = node == "humn";
  } else {
    Monkey::Binary b = std::get<Monkey::Binary>(expr);
    Evaluate(monkeys, values, b.left);
    Value left = values.at(b.left);
    Evaluate(monkeys, values, b.right);
    Value right = values.at(b.right);
    switch (b.op) {
      case '+':
        v.value = left.value + right.value;
        break;
      case '-':
        v.value = left.value - right.value;
        break;
      case '/':
        v.value = left.value / right.value;
        break;
      case '*':
        v.value = left.value * right.value;
        break;
    }
    v.has_humn = left.has_humn || right.has_humn;
  }
  values[node] = v;
}

absl::flat_hash_map<std::string, Value> Evaluate(
    const absl::flat_hash_map<std::string, Monkey>& monkeys,
    absl::string_view node) {
  // By preallocating the right number of elements we appear to save a bunch of
  // time.
  absl::flat_hash_map<std::string, Value> values(monkeys.size());
  Evaluate(monkeys, values, node);
  return values;
}

int64_t FindHumn(const absl::flat_hash_map<std::string, Monkey>& monkeys,
                 const absl::flat_hash_map<std::string, Value>& values,
                 absl::string_view node, int64_t target) {
  if (node == "humn") {
    return target;
  }
  Monkey::Binary b = monkeys.at(node).MustBinary();
  const Value& left = values.at(b.left);
  const Value& right = values.at(b.right);
  std::string next_node = left.has_humn ? b.left : b.right;
  int64_t next_target = -1;
  if (left.has_humn) {
    //                       target = left (humn) <op> right
    // => target <reverse op> right = left (humn)
    int64_t r = right.value;
    switch (b.op) {
      case '+':
        next_target = target - r;
        break;
      case '-':
        next_target = target + r;
        break;
      case '/':
        next_target = target * r;
        break;
      case '*':
        next_target = target / r;
        break;
    }
  } else {
    int64_t l = left.value;
    switch (b.op) {
      case '+':
        //           target = left + right (humn)
        // => target - left = right (humn)
        next_target = target - l;
        break;
      case '-':
        //                   target = left - right (humn)
        // => target + right (humn) = left
        // =>          right (humn) = left - target
        next_target = l - target;
        break;
      case '/':
        //                   target = left / right (humn)
        // => target * right (humn) = left
        // =>          right (humn) = left / target
        next_target = l / target;
        break;
      case '*':
        //           target = left * right (humn)
        // => target / left = right (humn)
        next_target = target / l;
        break;
    }
  }
  return FindHumn(monkeys, values, next_node, next_target);
}

int64_t FindHumn(const absl::flat_hash_map<std::string, Monkey>& monkeys,
                 const absl::flat_hash_map<std::string, Value>& values) {
  Monkey::Binary root = monkeys.at("root").MustBinary();
  const Value& left = values.at(root.left);
  const Value& right = values.at(root.right);
  std::string node;
  int64_t target;
  if (!left.has_humn) {
    target = left.value;
    node = root.right;
  } else {
    target = right.value;
    node = root.left;
  }
  return FindHumn(monkeys, values, node, target);
}

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  absl::flat_hash_map<std::string, Monkey> monkeys = Parse(input);
  absl::flat_hash_map<std::string, Value> values = Evaluate(monkeys, "root");
  if (part1) {
    return std::to_string(values.at("root").value);
  }
  return std::to_string(FindHumn(monkeys, values));
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}
}  // namespace day21
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
