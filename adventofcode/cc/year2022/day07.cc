#include "adventofcode/cc/year2022/day07.h"

#include <algorithm>
#include <optional>
#include <string>
#include <vector>

#include "absl/status/statusor.h"
#include "absl/strings/numbers.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day07 {
namespace {

class Node {
 public:
  Node() : Node("") {}
  Node(std::string name) : name_(name), size_(std::nullopt) {}
  Node(std::string name, int size) : name_(name), size_(size) {}

  std::string Name() const { return name_; }
  bool IsDir() const { return Size() == std::nullopt; }
  std::optional<int> Size() const { return size_; }

  std::optional<Node*> Child(absl::string_view name) const {
    for (Node* child : children_) {
      if (child->Name() == name) {
        return child;
      }
    }
    return std::nullopt;
  }

  std::vector<Node*> Children() const { return children_; }

  Node* AddChild(absl::string_view name) {
    std::string name_str(name);
    Node* child = new Node(name_str);
    children_.push_back(child);
    return child;
  }

  Node* AddChild(absl::string_view name, int size) {
    std::string name_str(name);
    Node* child = new Node(name_str, size);
    children_.push_back(child);
    return child;
  }

 private:
  std::string name_;
  std::optional<int> size_;

  std::vector<Node*> children_;
};

absl::StatusOr<Node*> parse(absl::string_view input) {
  Node* root = new Node();
  std::vector<Node*> pwd = {root};

  std::vector<absl::string_view> lines = absl::StrSplit(input, '\n');
  for (absl::string_view line : lines) {
    if (line == "") {
      // End of input.
      break;
    }
    if (line[0] == '$') {
      // We assume that this is a command, i.e., line[0] == '$'.  Commands have
      // one of two following forms:
      //     $ cd <arg>   // 1
      //     $ ls         // 2
      // So we know that if line[2] == 'c', it is a cd command, and the argument
      // is from line[5] and to the end; if line[2] == 'l' we know it is an ls
      // command and we need to begin parsing its output.
      if (line[2] == 'l') {
        // This is an ls command. We just skip past it.
        continue;
      }
      // This is a cd command.
      //
      // Assumption: we can only cd to the root (arg == "/"), the parent
      // directory (".."), or to a directory we have previously seen in ls
      // output.
      std::string arg(line.substr(5));
      if (arg == "/") {
        pwd = {root};
        continue;
      }
      if (arg == "..") {
        pwd.pop_back();
        continue;
      }
      std::optional<Node*> child = pwd.back()->Child(arg);
      if (!child.has_value()) {
        return absl::InternalError("cd'ing to " + arg +
                                   " which we haven't seen before");
      }
      if ((*child)->Size()) {
        return absl::InternalError("cd'ing to " + arg +
                                   " which is not a directory");
      }
      pwd.push_back(*child);
      continue;
    }
    // This is a line containing ls output.  The output has one of two forms:
    //     dir <dir_name>
    //     <size> <file_name>
    // We can distinguish between them by looking at the first character of
    // the line. If it is a 'd', it is the 'dir' form; otherwise it's the file
    // form.
    if (line[0] == 'd') {
      // Indices into the string:
      //     dir <dir_name>
      //     0123456789...
      //         ^ start of dirname
      // So we take the substring starting at index 4 and continuing to the
      // end of the line.
      absl::string_view dir_name = line.substr(4);
      // We assume we haven't seen this before.
      if (pwd.back()->Child(dir_name).has_value()) {
        return absl::InternalError("ls output contains dir " +
                                   std::string(dir_name) +
                                   " which we have seen before");
      }
      pwd.back()->AddChild(dir_name);
      continue;
    }
    // This should be a line of the file form. We find the index of the
    // space separating the size from the file name, and create substrings
    // based on it.
    size_t space = line.find(' ');
    absl::string_view size_str = line.substr(0, space);
    int size;
    if (!absl::SimpleAtoi(size_str, &size)) {
      return absl::InvalidArgumentError(std::string(size_str) +
                                        " couldn't be parsed as an integer");
    }
    absl::string_view file_name = line.substr(space + 1);
    // We assume we haven't seen this before.
    if (pwd.back()->Child(file_name).has_value()) {
      return absl::InternalError("ls output contains file " +
                                 std::string(file_name) +
                                 " which we have seen before");
    }
    pwd.back()->AddChild(file_name, size);
  }

  return root;
}

int sumSmallDirs(Node* root, int& sum) {
  if (!root->IsDir()) {
    return root->Size().value();
  }
  int size = 0;
  for (Node* child : root->Children()) {
    size += sumSmallDirs(child, sum);
  }
  if (size <= 100'000) {
    sum += size;
  }
  return size;
}

// sumSmallDirs traverses the file tree rooted at root and returns the sum of
// sizes of all directories with total sizes of less than 100 000.
int sumSmallDirs(Node* root) {
  int sum = 0;
  sumSmallDirs(root, sum);
  return sum;
}

// This helper function has a wonky signature. Basically, the return value is
// the size of the node (file size for files, total size for directories).
// Additionally, it accumulates the sizes of all directories in the sizes
// vector.
int buildSizes(Node* root, std::vector<int>& sizes) {
  if (!root->IsDir()) {
    return root->Size().value();
  }
  int size = 0;
  for (Node* child : root->Children()) {
    size += buildSizes(child, sizes);
  }
  sizes.push_back(size);
  return size;
}

// buildSizes calculates the total sizes for all directories in the tree rooted
// at root and returns them.
std::vector<int> buildSizes(Node* root) {
  std::vector<int> sizes;
  buildSizes(root, sizes);
  return sizes;
}

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  absl::StatusOr<Node*> root = parse(input);
  if (!root.ok()) {
    return root.status();
  }
  int answer;
  if (part1) {
    answer = sumSmallDirs(*root);
  } else {
    std::vector<int> sizes = buildSizes(*root);

    int capacity = 70000000;
    // The amount of used space is equal to the total size of the root
    // directory, and since that directory contains all other directories, it
    // will be the max element in the sizes vector.
    int used = *std::max_element(sizes.cbegin(), sizes.cend());
    int available = capacity - used;
    int needed = 30000000;
    int to_delete = needed - available;
    // We do a slight hack here to find the smallest element larger than
    // `to_delete`. C++17 (the standard I'm targeting here) doesn't have the
    // same niceties that Rust has with stuff like `.map(|size| size -
    // to_delete).filter(|d| d > 0).min()`. (There are some of those niceties in
    // C++20, though).
    // Instead, we "filter" by considering all values `< to_delete` to be larger
    // than all values `>= to_delete`.
    answer =
        *std::min_element(sizes.cbegin(), sizes.cend(),
                          [to_delete](const int& a, const int& b) -> bool {
                            // Since this is a min operation, this function
                            // implements the logic "is `a` less than `b`".
                            // We're trying to answer the question "is
                            // `a` a better candidate than `b`?", and so we let
                            // "`a` less than `b`" represent "`a` is a better
                            // candidate than `b`".
                            // * If `a` is smaller than `to_delete`, then the
                            //   answer is no.
                            // * If `b` is smaller than `to_delete`, then the
                            //   answer is yes.
                            // * If neither of them are big enough, we don't
                            //   care and can return whatever we want.
                            // * Otherwise, when both `a` and `b` are big
                            //   enough, we just compare `a < b`.
                            if (a < to_delete) {
                              return false;  // => a isn't a candidate at all,
                                             // so it can't be better than b
                            }
                            if (b < to_delete) {
                              return true;  // => a is a candidate but b isn't,
                                            // so a is obviously better than b
                            }
                            return a < b;
                          });
  }
  delete *root;
  return std::to_string(answer);
}

}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}

}  // namespace day07
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
