#include "adventofcode/cc/year2022/day13.h"

#include <algorithm>
#include <string>
#include <utility>
#include <vector>

#include "absl/log/check.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"
#include "adventofcode/cc/trim.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day13 {
namespace {

class Value {
 public:
  static Value Integer(int i) { return Value(i); }
  static Value List(std::vector<Value> elements) { return Value(elements); }
  static Value Parse(absl::string_view s) { return Parse(s, 0).first; }

  // Assumes is_integer() == true.
  int integer() const { return i_; }
  bool is_integer() const { return !is_list(); }
  // Assumes is_list() == true.
  std::vector<Value> list() const { return elements_; }
  bool is_list() const { return is_list_; }

  std::string String() const {
    if (is_integer()) {
      return std::to_string(integer());
    }
    std::string s = "[";
    std::vector<Value> elements = list();
    for (size_t i = 0; i < elements.size(); i++) {
      if (i > 0) {
        s += ",";
      }
      s += elements[i].String();
    }
    s += "]";
    return s;
  }

  // Returns < 0 if left comes before right.
  // Returns > 0 if right comes before left.
  // Returns = 0 if order cannot be determined.
  static int Compare(const Value& left, const Value& right) {
    if (left.is_integer() && right.is_integer()) {
      return left.integer() - right.integer();
    }
    if (left.is_list() && right.is_list()) {
      std::vector<Value> left_list = left.list();
      std::vector<Value> right_list = right.list();
      for (size_t i = 0; i < left_list.size() && i < right_list.size(); i++) {
        int cmp = Compare(left_list[i], right_list[i]);
        if (cmp != 0) {
          return cmp;
        }
      }
      // The lists have a shared prefix. We can use the length of the lists to
      // implement the rest of the logic.
      return int(left_list.size()) - int(right_list.size());
    }
    // Exactly one of the values is an integer; convert it to a list and then
    // redo the comparison.
    Value left2 = left.is_list() ? left : List({Integer(left.integer())});
    Value right2 = right.is_list() ? right : List({Integer(right.integer())});
    return Compare(left2, right2);
  }

  inline bool operator<(const Value& other) const {
    return Compare(*this, other) < 0;
  }

 private:
  static std::pair<Value, size_t> Parse(absl::string_view s, size_t pos) {
    // Assumption: s[pos] is either the start of an integer or the start of a
    // list.
    if (s[pos] == '[') {
      return ParseList(s, pos);
    } else {
      return ParseInteger(s, pos);
    }
  }

  static std::pair<Value, size_t> ParseList(absl::string_view s, size_t pos) {
    std::vector<Value> elements;
    // Assumption: s[pos] is a '[' indicating the start of a list.
    pos++;
    while (s[pos] != ']') {
      auto [element, next] = Parse(s, pos);
      elements.push_back(element);
      pos = next;
      if (s[pos] == ',') {
        pos++;
      }
    }
    // s[pos] == ']', so we must advance past it.
    pos++;
    return std::make_pair(List(elements), pos);
  }

  static std::pair<Value, size_t> ParseInteger(absl::string_view s,
                                               size_t pos) {
    // Assumption: s[pos] is a digit.
    int i;
    size_t next = s.find_first_not_of("0123456789", pos);
    CHECK(absl::SimpleAtoi(s.substr(pos, next - pos), &i));
    return std::make_pair(Integer(i), next);
  }

  Value(int i) : i_(i), is_list_(false) {}
  Value(std::vector<Value> elements) : elements_(elements), is_list_(true) {}

  int i_;
  std::vector<Value> elements_;
  bool is_list_;
};
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  // We're going to split on double newlines, which means that unless we trim
  // off the whitespace, the last "fragment" will have its second line
  // terminated by a newline -- which is not what we want.
  input = adventofcode::cc::trim::TrimSpace(input);
  std::vector<absl::string_view> fragments =
      absl::StrSplit(input, "\n\n", absl::SkipEmpty());
  int sum = 0;
  for (size_t i = 0; i < fragments.size(); i++) {
    absl::string_view fragment = fragments[i];
    size_t newline = fragment.find('\n');
    Value left = Value::Parse(fragment.substr(0, newline));
    Value right = Value::Parse(fragment.substr(newline + 1));
    if (Value::Compare(left, right) < 0) {
      sum += i + 1;
    }
  }
  return std::to_string(sum);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  std::vector<Value> values;
  for (absl::string_view line :
       absl::StrSplit(input, '\n', absl::SkipEmpty())) {
    values.push_back(Value::Parse(line));
  }
  absl::string_view decoder1 = "[[2]]";
  absl::string_view decoder2 = "[[6]]";
  values.push_back(Value::Parse(decoder1));
  values.push_back(Value::Parse(decoder2));
  std::sort(values.begin(), values.end());
  int decoder_key = 1;
  for (size_t i = 0; i < values.size(); i++) {
    std::string s = values[i].String();
    if (s == decoder1 || s == decoder2) {
      decoder_key *= i + 1;
    }
  }
  return std::to_string(decoder_key);
}

}  // namespace day13
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
