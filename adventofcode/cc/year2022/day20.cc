#include "adventofcode/cc/year2022/day20.h"

#include <cstdint>
#include <string>
#include <vector>

#include "absl/log/check.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day20 {
namespace {

struct Node {
  int64_t v;
  Node* prev;
  Node* next;
};

void MoveNode(Node* n, int64_t node_count) {
  int64_t steps = n->v;
  int64_t gap_count = node_count - 1;
  while (steps < 0) {
    steps += gap_count;
  }
  steps %= gap_count;
  if (steps == 0) {
    return;
  }
  Node* dst = n;
  // Move n forwards by swapping it with its next node.
  for (int64_t i = 0; i < steps; i++) {
    dst = dst->next;
  }
  // Detach n.
  n->prev->next = n->next;
  n->next->prev = n->prev;
  // Insert n _after_ dst.
  n->prev = dst;
  n->next = dst->next;
  n->prev->next = n;
  n->next->prev = n;
}

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  // The numbers are parsed into a circular linked list by first creating the
  // nodes in the list without any connections between them, and then creating
  // the connections. That way we don't have to deal with a bunch of edge cases
  // around empty lists or 1-element lists.
  std::vector<Node*> nodes;
  Node* zero = nullptr;
  for (absl::string_view line :
       absl::StrSplit(input, '\n', absl::SkipEmpty())) {
    Node* n = new Node;
    CHECK(absl::SimpleAtoi(line, &n->v)) << line;
    nodes.push_back(n);
    if (n->v == 0) {
      zero = n;
    }
  }
  CHECK(zero != nullptr) << "No zero node found";
  for (size_t i = 0; i < nodes.size(); i++) {
    size_t prev_i = (i == 0) ? nodes.size() - 1 : i - 1;
    size_t next_i = (i == nodes.size() - 1) ? 0 : i + 1;
    nodes[i]->prev = nodes[prev_i];
    nodes[i]->next = nodes[next_i];
  }

  if (!part1) {
    constexpr int64_t kDecryptionKey = 811589153;
    for (Node* n : nodes) {
      n->v *= kDecryptionKey;
    }
  }

  int rounds = part1 ? 1 : 10;
  for (int i = 0; i < rounds; i++) {
    for (Node* n : nodes) {
      MoveNode(n, nodes.size());
    }
  }

  int64_t sum = 0;
  Node* n = zero;
  for (int i = 1; i <= 3000; i++) {
    n = n->next;
    if (i % 1000 == 0) {
      sum += n->v;
    }
  }

  for (Node* n : nodes) {
    delete n;
  }
  return std::to_string(sum);
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}
}  // namespace day20
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
