#include "adventofcode/cc/year2022/day01.h"

#include <string>

#include "benchmark/benchmark.h"
#include "runfiles/cc/runfiles.h"

static std::string argv0;

static void BM_Part1(benchmark::State& state) {
  const auto input = runfiles::cc::runfiles::Read(
      "code/adventofcode/data/year2022/day01.real.in", argv0);
  if (!input.ok()) {
    const std::string error(input.status().message());
    state.SkipWithError(error.c_str());
    return;
  }
  for (auto _ : state) {
    benchmark::DoNotOptimize(adventofcode::cc::year2022::day01::Part1(*input));
  }
}
BENCHMARK(BM_Part1);

static void BM_Part2(benchmark::State& state) {
  const auto input = runfiles::cc::runfiles::Read(
      "code/adventofcode/data/year2022/day01.real.in", argv0);
  if (!input.ok()) {
    const std::string error(input.status().message());
    state.SkipWithError(error.c_str());
    return;
  }
  for (auto _ : state) {
    benchmark::DoNotOptimize(adventofcode::cc::year2022::day01::Part2(*input));
  }
}
BENCHMARK(BM_Part2);

int main(int argc, char** argv) {
  argv0 = argv[0];
  benchmark::Initialize(&argc, argv);
  benchmark::RunSpecifiedBenchmarks();
  benchmark::Shutdown();
}
