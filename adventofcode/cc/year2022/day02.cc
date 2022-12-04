#include "adventofcode/cc/year2022/day02.h"

#include <string>

#include "absl/status/statusor.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day02 {
namespace {
class Choice {
 public:
  enum Value {
    kRock,
    kPaper,
    kScissors,
  };

  Value value;

  Choice() = default;
  constexpr Choice(Value v) : value(v) {}

  const std::string String() const {
    switch (value) {
      case kRock:
        return "rock";
      case kPaper:
        return "paper";
      case kScissors:
        return "rock";
    }
  }

  bool operator==(const Choice& other) const { return value == other.value; }

  Choice WinsAgainst() const {
    switch (value) {
      case kRock:
        return Choice(kScissors);
      case kPaper:
        return Choice(kRock);
      case kScissors:
        return Choice(kPaper);
    }
  }

  Choice LosesAgainst() const {
    switch (value) {
      case kRock:
        return Choice(kPaper);
      case kPaper:
        return Choice(kScissors);
      case kScissors:
        return Choice(kRock);
    }
  }

  // Against returns the winner of this Choice and the other. It returns -1 if
  // this Choice loses, 0 if it's a draw, and +1 if this Choice wins.
  int Against(const Choice& other) const {
    if (value == other.value) {
      return 0;
    }
    if (WinsAgainst() == other) {
      return 1;
    }
    return -1;
  }
};

class Round {
 public:
  Choice my_choice;
  Choice opponent_choice;

  static absl::StatusOr<Round> Parse(absl::string_view line, bool part2) {
    Round r;
    if (!part2) {
      // line[0] = first character = opponents choice
      // line[1] = space
      // line[2] = second character = my choice
      switch (line[0]) {
        case 'A':
          r.opponent_choice = Choice::kRock;
          break;
        case 'B':
          r.opponent_choice = Choice::kPaper;
          break;
        case 'C':
          r.opponent_choice = Choice::kScissors;
          break;
        default:
          return absl::InvalidArgumentError("invalid first character: " +
                                            std::string(line));
      }

      switch (line[2]) {
        case 'X':
          r.my_choice = Choice::kRock;
          break;
        case 'Y':
          r.my_choice = Choice::kPaper;
          break;
        case 'Z':
          r.my_choice = Choice::kScissors;
          break;
        default:
          return absl::InvalidArgumentError("invalid second character: " +
                                            std::string(line));
      }
    } else {
      // line[0] = first character = opponent choice
      // line[1] = space
      // line[2] = second character = outcome (X = lose, Y = draw, Z = win)
      switch (line[0]) {
        case 'A':
          r.opponent_choice = Choice::kRock;
          break;
        case 'B':
          r.opponent_choice = Choice::kPaper;
          break;
        case 'C':
          r.opponent_choice = Choice::kScissors;
          break;
        default:
          return absl::InvalidArgumentError("invalid first character: " +
                                            std::string(line));
      }

      switch (line[2]) {
        case 'X':  // lose
          r.my_choice = r.opponent_choice.WinsAgainst();
          break;
        case 'Y':  // draw
          r.my_choice = r.opponent_choice;
          break;
        case 'Z':  // win
          r.my_choice = r.opponent_choice.LosesAgainst();
          break;
        default:
          return absl::InvalidArgumentError("invalid second character: " +
                                            std::string(line));
      }
    }
    return r;
  }

  // Score returns the score of this round, according to the rules explained in
  // the problem description.
  int Score() const {
    int score = 0;
    switch (my_choice.value) {
      case Choice::kRock:
        score += 1;
        break;
      case Choice::kPaper:
        score += 2;
        break;
      case Choice::kScissors:
        score += 3;
        break;
    }
    switch (my_choice.Against(opponent_choice)) {
      case -1:
        score += 0;
        break;
      case 0:
        score += 3;
        break;
      case 1:
        score += 6;
        break;
    }
    return score;
  }
};

absl::StatusOr<std::string> solve(absl::string_view input, bool part2) {
  int score = 0;
  for (auto line : absl::StrSplit(input, '\n')) {
    if (line == "") {
      // Should only be true for last line (assuming trailing newline in input),
      // so assume we reached the end and break.
      break;
    }
    const auto round = Round::Parse(line, part2);
    if (!round.ok()) {
      return absl::Status(round.status());
    }
    score += round->Score();
  }
  return std::to_string(score);
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part2=*/false);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part2=*/true);
}
}  // namespace day02
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
