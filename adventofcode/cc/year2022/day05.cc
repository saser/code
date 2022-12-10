#include "adventofcode/cc/year2022/day05.h"

// Potential optimization ideas:
// * experiment with using a std::deque rather than a std::vector
// * remove absl::StrSplit and iterate through lines by searching for newline
//   characters
// * move all parsing into the same method, so that parsing the instructions can
//   benefit from knowing where the crates part of the input ends

#include <string>
#include <vector>

#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day05 {
namespace {

struct Instruction {
  int n;
  int from;  // 1-indexed
  int to;    // 1-indexed
};

// parseInstruction parses a single instruction line into an Instruction.
absl::StatusOr<Instruction> parseInstruction(absl::string_view line) {
  Instruction i;
  std::vector<absl::string_view> parts = absl::StrSplit(line, ' ');
  // The line looks like:
  //     move NNNN from X to Y
  //     0    1    2    3 4  5 = indices into parts
  if (!absl::SimpleAtoi(parts[1], &i.n)) {
    return absl::InvalidArgumentError("invalid line (part 1): " +
                                      std::string(line));
  }
  if (!absl::SimpleAtoi(parts[3], &i.from)) {
    return absl::InvalidArgumentError("invalid line (part 3): " +
                                      std::string(line));
  }
  if (!absl::SimpleAtoi(parts[5], &i.to)) {
    return absl::InvalidArgumentError("invalid line (part 3): " +
                                      std::string(line));
  }
  return i;
}

// parseAllInstructions parses all instructions from the input. It figures out
// where the instructions start and then starts parsing them line by line.
absl::StatusOr<std::vector<Instruction>> parseAllInstructions(
    absl::string_view input) {
  // We can abuse the fact that all crates have uppercase letters, and then look
  // for the first occurrence of an 'm' character (being the first character in
  // the first "move" word). When we find it we can crop the input to contain
  // only the instructions.
  input = input.substr(input.find('m'));
  std::vector<Instruction> instructions;
  for (absl::string_view line : absl::StrSplit(input, '\n')) {
    if (line == "") {
      // End of input.
      break;
    }
    absl::StatusOr<Instruction> instr = parseInstruction(line);
    if (!instr.ok()) {
      return absl::Status(instr.status());
    }
    instructions.push_back(*instr);
  }
  return instructions;
}

struct Cargo {
  // A 9-element array of stacks.
  std::vector<char> stacks[9];

  void Apply(const Instruction& instr, bool one_at_a_time) {
    // We need to convert from the 1-indexed instruction to the 0-indexed
    // array.
    std::vector<char>& from = stacks[instr.from - 1];
    std::vector<char>& to = stacks[instr.to - 1];
    if (one_at_a_time) {
      // Push to destination in reverse order.
      for (auto it = from.crbegin(); it != from.crbegin() + instr.n; it++) {
        to.push_back(*it);
      }
    } else {
      // Push to destination in existing order.
      for (auto it = from.end() - instr.n; it != from.end(); it++) {
        to.push_back(*it);
      }
    }
    // Crop the source to no longer include the moved elements.
    from.resize(from.size() - instr.n);
  }

  std::string TopCrates() const {
    std::string top;
    for (size_t i = 0; i < 9; i++) {
      std::vector<char> stack = stacks[i];
      if (stack.empty()) {
        continue;
      }
      top += stack.back();
    }
    return top;
  }

  std::string DebugString() const {
    std::string s;
    for (size_t i = 0; i < 9; i++) {
      for (const char& c : stacks[i]) {
        s += '[';
        s += c;
        s += ']';
        s += ' ';
      }
      s += '\n';
    }
    return s;
  }
};

// parseCargo takes input that _at least_ covers the initial stacks and the line
// with the numbers after them. In other words, it's possible to pass the entire
// input string to this function.
absl::StatusOr<Cargo> parseCargo(absl::string_view input) {
  // Assumption: the input begins like this:
  //
  //         [Q] [B]             [H]
  //         [F] [W] [D] [Q]     [S]
  //         [D] [C] [N] [S] [G] [F]
  //         [R] [D] [L] [C] [N] [Q]     [R]
  //     [V] [W] [L] [M] [P] [S] [M]     [M]
  //     [J] [B] [F] [P] [B] [B] [P] [F] [F]
  //     [B] [V] [G] [J] [N] [D] [B] [L] [V]
  //     [D] [P] [R] [W] [H] [R] [Z] [W] [S]
  //      1   2   3   4   5   6   7   8   9
  //
  //               1111111111222222222233333
  //     01234567890123456789012345678901234
  //     ^^^ string positions into each _line_
  //
  // There is some regularities here we can abuse:
  //
  // A. The letters can only appear on certain positions within a line, and
  //    these positions are evenly spaced apart.
  // B. The lines with the stacks contain only whitespace, brackets ("[]") and
  //    letters.
  // C. The line with the numbers contain only numbers.
  //
  // From (A) we know that if we can find the string position of any letter
  // within a line, we can with simple arithmetic calculate which stack it is
  // part of. In the figure above, we can see that:
  //
  //     stack 1 = pos 1
  //     stack 2 = pos 5
  //     stack 3 = pos 9
  //     ...
  //     stack 9 = pos 33
  //
  // The distances are 4 between each stack, and they are offset by 1. This
  // means that if we have a position N that we know is on a letter, the stack
  // can be calculated as
  //
  //     stack := ((N-1) / 4) + 1
  //
  // and abusing the fact that we can do integer division with computers, it's
  // just
  //
  //     stack := (N / 4) + 1.
  //
  // We also store the stacks using 0-indexing (meaning: stacks[0] is stack 1,
  // stacks[1] is stack 2, etc). We can therefore remove the +1 part as well and
  // end up with:
  //
  //     stack := N / 4.
  //
  // From (B) we know that we can either look for brackets and know that the
  // subsequent/previous character is a letter, or look for any uppercase
  // letter.
  //
  // From (C) we know that we can look for the first occurrence of a '1'
  // character and "crop" the part of the input we care about to contain only
  // the stacks and the numbers.
  // --------------------------------------------------------------------------
  // From this we build the following parsing strategy:
  // 1. Crop the input to the part we care about.
  // 2. Start from the beginning of the cropped input, and begin searching for
  //    opening brackets ('[') or newlines ('\n'). Keep track of the absolute
  //    position (i.e., relative to start of input) of where the current line
  //    starts.
  //    a. If we find an opening bracket, we know that the next character is a
  //       letter. Calculate which stack it belongs to using arithmetic.
  //    b. If we find a newline, advance past it and reset the "position in
  //       line" variable.
  // 3. Keep doing this for the remainder of the input. Once we can no longer
  //    find opening brackets or newlines, we are done.
  Cargo c;

  // First, let's crop the string.
  {
    size_t end = input.find('1');
    // We assume that the string looks like this:
    //
    //    [...]   '\n'   ' '   '1'
    //                          ^ end points here
    //
    // We move end back to before the newline, which means taking 3 steps back:
    //
    //    1 step:
    //    [...]   '\n'   ' '   '1'
    //                    ^ end
    //
    //    2 steps:
    //    [...]   '\n'   ' '   '1'
    //             ^ end
    //
    //    3 steps:
    //    [...]   '\n'   ' '   '1'
    //      ^ end (last character before newline)
    end -= 3;
    // Now we can use this to crop the input from the beginning up to _and
    // including_ end (that's the +1).
    input = input.substr(0, end + 1);
  }

  // Now we start searching through the string from beginning to end.
  size_t pos = 0;         // Where in the string we are.
  size_t line_start = 0;  // Absolute position of start of current line;
  size_t next;            // Where the next '[' or '\n' character is.
  for (;;) {
    next = input.find_first_of("[\n", pos);
    if (next == absl::string_view::npos) {
      break;
    }
    // Set current position to just after whatever we found.
    pos = next + 1;
    // What character did we find?
    switch (input[next]) {
      case '[': {  // We need {} here because we're introducing new variables.
        // pos points to a letter.
        size_t n = pos - line_start;  // This is N in the formula above.
        // The & is important, otherwise we get a copy!
        std::vector<char>& stack = c.stacks[n / 4];
        stack.insert(stack.begin(), input[pos]);  // NOTE: may be expensive.
        break;
      }
      case '\n':
        // pos points to the first character on the new line.
        line_start = pos;
        break;
    }
  }
  return c;
}

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  absl::StatusOr<Cargo> cargo = parseCargo(input);
  if (!cargo.ok()) {
    return absl::Status(cargo.status());
  }
  absl::StatusOr<std::vector<Instruction>> instructions =
      parseAllInstructions(input);
  if (!instructions.ok()) {
    return absl::Status(instructions.status());
  }
  for (const Instruction& instr : *instructions) {
    cargo->Apply(instr, part1);
  }
  return cargo->TopCrates();
}

}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}

}  // namespace day05
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
