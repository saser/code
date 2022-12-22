#include "adventofcode/cc/year2022/day16.h"

#include <algorithm>
#include <cstdint>
#include <string>
#include <utility>
#include <vector>

#include "absl/container/flat_hash_map.h"
#include "absl/container/flat_hash_set.h"
#include "absl/log/check.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/str_format.h"
#include "absl/strings/str_join.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"
#include "re2/re2.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day16 {
namespace {
struct Cave {
  std::string name;
  int flow_rate;
  std::vector<std::string> connections;

  static Cave Parse(absl::string_view line) {
    Cave c;
    std::string connections_str;
    static RE2 re(
        R"(Valve ([A-Z]+) has flow rate=(\d+); tunnels? leads? to valves? ([A-Z ,]+))");
    CHECK(RE2::FullMatch(line, re, &c.name, &c.flow_rate, &connections_str));
    c.connections = absl::StrSplit(connections_str, ", ");
    return c;
  }

  std::string String() const {
    return absl::StrFormat(
        "Valve %s has flow rate=%d; tunnels lead to valves %s", name, flow_rate,
        absl::StrJoin(connections, ", "));
  }
};

// MaxFlows constructs a map from a bitset of visited valves to the maximum flow
// that can be created by visiting those valves in some order.
void MaxFlows(
    // Where we accumulate the return value.
    absl::flat_hash_map<int64_t, int64_t>& output,
    // Current cave.
    int64_t current_cave,
    // How many minutes are remaining.
    int64_t minutes_remaining,
    // The total flow in this configuration.
    int64_t total_flow,
    // Bitset of which valves are currently open (i.e., we shouldn't visit
    // them). This is the "state" and will be used as the key in the return
    // value.
    int64_t open_valves,
    // Which valves we should consider visiting and their flow rates.
    const absl::flat_hash_map<int64_t, int64_t>& non_zero_caves,
    // dist[i][j] = length of shortest path from i to j.
    const absl::flat_hash_map<int64_t, absl::flat_hash_map<int64_t, int64_t>>&
        dist) {
  if (!output.contains(open_valves)) {
    output[open_valves] = total_flow;
  }
  output[open_valves] = std::max(output[open_valves], total_flow);
  for (const auto& [cave, flow_rate] : non_zero_caves) {
    int64_t bitmask = int64_t(1) << cave;
    if ((open_valves & bitmask) != 0) {
      // We have already visited that cave.
      continue;
    }
    // It takes dist[src][dst] minutes to travel there + 1 minute to open the
    // valve.
    int64_t minutes_required = dist.at(current_cave).at(cave) + 1;
    if (minutes_required >= minutes_remaining) {
      // It's either impossible or meaningless to go visit the cave.
      continue;
    }
    int64_t new_current_cave = cave;
    int64_t new_minutes_remaining = minutes_remaining - minutes_required;
    int64_t new_total_flow = total_flow + flow_rate * new_minutes_remaining;
    int64_t new_open_valves = open_valves | bitmask;
    MaxFlows(output, new_current_cave, new_minutes_remaining, new_total_flow,
             new_open_valves, non_zero_caves, dist);
  }
}

// MaxFlows returns a vector of mapping from configuration to max flow
// achievable with that configuration.
std::vector<std::pair<int64_t, int64_t>> MaxFlows(
    // Which cave we start in.
    int64_t starting_cave,
    // The total amount of time available.
    int64_t minutes,
    // Which valves we should consider visiting and their flow rates.
    const absl::flat_hash_map<int64_t, int64_t>& non_zero_caves,
    // dist[i][j] = length of shortest path from i to j.
    const absl::flat_hash_map<int64_t, absl::flat_hash_map<int64_t, int64_t>>&
        dist) {
  absl::flat_hash_map<int64_t, int64_t> output;
  MaxFlows(output, starting_cave, minutes,
           0,  // We assume we start in a cave with zero flow.
           0,  // There are currently no open valves.
           non_zero_caves, dist);
  return std::vector<std::pair<int64_t, int64_t>>(output.cbegin(),
                                                  output.cend());
}

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  // Parse the input into a map of cave number -> cave, as well as cave name ->
  // cave number.
  std::vector<absl::string_view> lines =
      absl::StrSplit(input, '\n', absl::SkipEmpty());
  absl::flat_hash_map<int64_t, Cave> caves;
  absl::flat_hash_map<std::string, int64_t> cave_numbers;
  for (size_t i = 0; i < lines.size(); i++) {
    absl::string_view line = lines[i];
    Cave c = Cave::Parse(line);
    caves[i] = c;
    cave_numbers[c.name] = i;
  }

  // Floyd-Warshall
  // (https://en.wikipedia.org/wiki/Floyd%E2%80%93Warshall_algorithm) to find
  // all shortest paths.
  absl::flat_hash_map<int64_t, absl::flat_hash_map<int64_t, int64_t>> dist;
  for (const auto& [i, cave] : caves) {
    dist[i][i] = 0;
    for (const std::string& other : cave.connections) {
      int64_t j = cave_numbers[other];
      dist[i][j] = 1;
    }
  }
  int64_t cave_count = caves.size();
  for (int64_t k = 0; k < cave_count; k++) {
    for (int64_t i = 0; i < cave_count; i++) {
      for (int64_t j = 0; j < cave_count; j++) {
        if (!(dist[i].contains(k) && dist[k].contains(j))) {
          continue;
        }
        int64_t through_k = dist[i][k] + dist[k][j];
        if (!dist[i].contains(j) || dist[i][j] > through_k) {
          dist[i][j] = through_k;
        }
      }
    }
  }
  for (const auto& [src, dsts] : dist) {
    CHECK_EQ(dsts.size(), cave_count);
  }

  // We only care about the caves in which the valve has a non-zero flow rate.
  absl::flat_hash_map<int64_t, int64_t> non_zero_caves;
  for (const auto& [i, cave] : caves) {
    if (cave.flow_rate > 0) {
      non_zero_caves[i] = cave.flow_rate;
    }
  }

  int64_t minutes = part1 ? 30 : 26;
  std::vector<std::pair<int64_t, int64_t>> max_flows =
      MaxFlows(cave_numbers["AA"], minutes, non_zero_caves, dist);
  int64_t max = 0;
  if (part1) {
    for (const auto& [_, flow] : max_flows) {
      max = std::max(max, flow);
    }
  } else {
    // We don't have to simulate the two characters moving simultaneously.
    // Instead, we can find all possible configurations reachable in 26 minutes,
    // and find pairs of configurations in which there is no overlap in the set
    // of visited caves. Those pairs are the configurations possible for the
    // player and the elephant to visit together. We can then simply take the
    // sum of the flows to find the total flow, and find the max sum.
    //
    // I didn't come up with this solution on my own. Instead, I looked at the
    // Reddit thread for inspiration
    // (https://www.reddit.com/r/adventofcode/comments/zn6k1l/2022_day_16_solutions/).
    // I feel stupid in hindsight.
    for (const auto& [state1, flow1] : max_flows) {
      for (const auto& [state2, flow2] : max_flows) {
        if ((state1 & state2) != 0) {
          // There was some overlap in the caves.
          continue;
        }
        max = std::max(max, flow1 + flow2);
      }
    }
  }
  return std::to_string(max);
}
}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}
}  // namespace day16
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
