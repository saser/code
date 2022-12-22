#include "adventofcode/cc/year2022/day19.h"

#include <algorithm>
#include <cstdint>
#include <string>
#include <utility>

#include "absl/container/flat_hash_map.h"
#include "absl/log/check.h"
#include "absl/status/status.h"
#include "absl/status/statusor.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"
#include "re2/re2.h"

namespace adventofcode {
namespace cc {
namespace year2022 {
namespace day19 {
namespace {

// The various optimizations in this solution are ones I didn't come up with
// myself. Instead, I got them from this _fantastic_ video:
// https://www.youtube.com/watch?v=5rb0vvJ7NCY. All credit to that author -- I
// just internalized his ideas and translated them into code here.

struct Blueprint {
  uint8_t id;
  uint8_t ore;                           // Unit: ore.
  uint8_t clay;                          // Unit: ore.
  std::pair<uint8_t, uint8_t> obsidian;  // Units: <ore, clay>.
  std::pair<uint8_t, uint8_t> geode;     // Units: <ore, obsidian>.

  uint8_t max_ore_cost;
  uint8_t max_clay_cost;
  uint8_t max_obsidian_cost;

  static Blueprint Parse(absl::string_view line) {
    // For some reason that I haven't cared to debug, RE2 can't FullMatch into
    // the uint8_t values in Blueprint. So we set up corresponding int
    // variables here, parse into them, then feed them to the Blueprint below.
    int id;
    int ore;
    int clay;
    std::pair<int, int> obsidian;
    std::pair<int, int> geode;
    static RE2 blueprint_regex(
        R"(Blueprint (\d+): Each ore robot costs (\d+) ore. Each clay robot costs (\d+) ore. Each obsidian robot costs (\d+) ore and (\d+) clay. Each geode robot costs (\d+) ore and (\d+) obsidian.)");
    CHECK(RE2::FullMatch(line, blueprint_regex, &id, &ore, &clay,
                         &obsidian.first, &obsidian.second, &geode.first,
                         &geode.second))
        << line;
    Blueprint b{
        .id = uint8_t(id),
        .ore = uint8_t(ore),
        .clay = uint8_t(clay),
        .obsidian = std::make_pair(obsidian.first, obsidian.second),
        .geode = std::make_pair(geode.first, geode.second),
    };
    b.max_ore_cost = std::max({b.ore, b.clay, b.obsidian.first, b.geode.first});
    b.max_clay_cost = b.obsidian.second;
    b.max_obsidian_cost = b.geode.second;
    return b;
  }
};

struct State {
  // How many minutes have passed.
  uint8_t minutes;

  // How many robots of each kind we have.
  uint8_t ore_robots;
  uint8_t clay_robots;
  uint8_t obsidian_robots;
  uint8_t geode_robots;

  // How much resources we have.
  uint8_t ore;
  uint8_t clay;
  uint8_t obsidian;
  uint8_t geodes;

  inline bool CanBuildOreRobot(const Blueprint& b) const {
    return ore >= b.ore;
  }

  inline bool CanBuildClayRobot(const Blueprint& b) const {
    return ore >= b.clay;
  }

  inline bool CanBuildObsidianRobot(const Blueprint& b) const {
    return ore >= b.obsidian.first && clay >= b.obsidian.second;
  }

  inline bool CanBuildGeodeRobot(const Blueprint& b) const {
    return ore >= b.geode.first && obsidian >= b.geode.second;
  }

  inline State BuildOreRobot(const Blueprint& b) const {
    State s = *this;
    s.ore -= b.ore;
    s.ore_robots++;
    return s;
  }

  inline State BuildClayRobot(const Blueprint& b) const {
    State s = *this;
    s.ore -= b.clay;
    s.clay_robots++;
    return s;
  }

  inline State BuildObsidianRobot(const Blueprint& b) const {
    State s = *this;
    s.ore -= b.obsidian.first;
    s.clay -= b.obsidian.second;
    s.obsidian_robots++;
    return s;
  }

  inline State BuildGeodeRobot(const Blueprint& b) const {
    State s = *this;
    s.ore -= b.geode.first;
    s.obsidian -= b.geode.second;
    s.geode_robots++;
    return s;
  }

  inline State Step() const {
    State s = *this;
    s.minutes++;
    s.ore += ore_robots;
    s.clay += clay_robots;
    s.obsidian += obsidian_robots;
    s.geodes += geode_robots;
    return s;
  }

  // CannotBeat determines whether this state cannot possibly result in more
  // geodes being produced than `max`. This is useful as an optimization: if
  // CannotBeat returns true, then there's no point in continuing to explore
  // this State.
  inline bool CannotBeat(uint8_t limit, uint8_t max) const {
    // While we don't know exactly how many geodes this state can produce at
    // best, we can compute an upper bound. That upper bound is the sum of:
    //
    // 1. The current number of geodes. This is simple: it's stored in the pack.
    //
    // 2. The current number of geode robots. Each robot will produce 1 geode
    // each remaining minute, so we take the number of remaining minutes
    // multiplied by the current number of geode robots.
    //
    // 3. A best-case scenario of future geode robots: we build a new geode
    // robot each minute. There's no guarantee we would be able to do that, but
    // we absolutely cannot do _better_ than that. This becomes an arithmetic
    // sum: if we for e.g. the next 3 minutes build 1 geode robot each minute,
    // they will produce 0 + 1 + 2 geodes, and so on. Note the off-by-one
    // situation here: if we have N remaining minutes, the robots will only
    // actually produce geodes for N-1 minutes. So we take the arithmetic sum
    // from 1 to N-1, which is (N*(N-1))/2.

    uint8_t remaining = limit - minutes;

    uint16_t upper_bound = geodes                              // #1
                           + geode_robots * remaining          // #2
                           + remaining * (remaining - 1) / 2;  // #3
    return upper_bound <= max;
  }
};

void MaxGeodes(
    State s, const Blueprint& b,
    // The maximum number of minutes.
    uint8_t limit,
    // The highest result we've seen so far.
    uint8_t& max,
    // Whether we are "allowed" to build one
    // of these robots this step. These can be false if this state was reached
    // by waiting, and we could have built a robot instead of waiting.
    bool allowed_ore, bool allowed_clay, bool allowed_obsidian) {
  if (s.minutes == limit) {
    max = std::max(max, s.geodes);
    return;
  }
  if (s.CannotBeat(limit, max)) {
    return;
  }
  State next = s.Step();
  if (s.CanBuildGeodeRobot(b)) {
    MaxGeodes(next.BuildGeodeRobot(b), b, limit, max, true, true, true);
    // Building a geode robot is the best thing we can do -- there's no reason
    // to explore other states.
    return;
  }
  bool new_allowed_ore = true;
  bool new_allowed_clay = true;
  bool new_allowed_obsidian = true;
  if (
      // Whether we are allowed to build after waiting.
      allowed_obsidian
      // No point in building more obsidian robots if we are already producing
      // enough each minute to build geode robots.
      && s.obsidian_robots < b.max_obsidian_cost
      // Do we even have the resources?
      && s.CanBuildObsidianRobot(b)) {
    new_allowed_obsidian = false;
    MaxGeodes(next.BuildObsidianRobot(b), b, limit, max, true, true, true);
  }
  if (
      // Whether we are allowed to build after waiting.
      allowed_clay
      // No point in building more obsidian robots if we are already producing
      // enough each minute to build obsidian robots.
      && s.clay_robots < b.max_clay_cost
      // Whether we have the resources.
      && s.CanBuildClayRobot(b)) {
    new_allowed_clay = false;
    MaxGeodes(next.BuildClayRobot(b), b, limit, max, true, true, true);
  }
  if (
      // Whether we are allowed to build after waiting.
      allowed_ore
      // No point in building more ore robots if we are already producing
      // enough each minute to build any other robot.
      && s.ore_robots < b.max_ore_cost
      // Whether we have the resources
      && s.CanBuildOreRobot(b)) {
    new_allowed_ore = false;
    MaxGeodes(next.BuildOreRobot(b), b, limit, max, true, true, true);
  }
  MaxGeodes(next, b, limit, max, new_allowed_ore, new_allowed_clay,
            new_allowed_obsidian);
}

uint8_t MaxGeodes(const Blueprint& b, uint8_t limit) {
  State s{};
  s.ore_robots = 1;
  uint8_t max = 0;
  MaxGeodes(s, b, limit, max, true, true, true);
  return max;
}

absl::StatusOr<std::string> solve(absl::string_view input, bool part1) {
  std::vector<Blueprint> blueprints;
  for (absl::string_view line :
       absl::StrSplit(input, '\n', absl::SkipEmpty())) {
    blueprints.push_back(Blueprint::Parse(line));
  }
  if (part1) {
    uint8_t limit = 24;
    uint16_t answer = 0;
    for (const Blueprint& b : blueprints) {
      answer += b.id * MaxGeodes(b, limit);
    }
    return std::to_string(answer);
  } else {
    uint8_t limit = 32;
    uint16_t answer = 1;
    for (auto it = blueprints.cbegin();
         it != blueprints.cend() && it != blueprints.cbegin() + 3; it++) {
      answer *= MaxGeodes(*it, limit);
    }
    return std::to_string(answer);
  }
}

}  // namespace

absl::StatusOr<std::string> Part1(absl::string_view input) {
  return solve(input, /*part1=*/true);
}

absl::StatusOr<std::string> Part2(absl::string_view input) {
  return solve(input, /*part1=*/false);
}
}  // namespace day19
}  // namespace year2022
}  // namespace cc
}  // namespace adventofcode
