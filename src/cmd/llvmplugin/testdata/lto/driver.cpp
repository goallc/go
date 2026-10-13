// Test-only harness; production calls GoALLCFullLTO inside cmd/link.
#include "GoALLCLTO.h"
#include "llvm/Support/CommandLine.h"
#include "llvm/Support/raw_ostream.h"
using namespace llvm;
static cl::list<std::string> Inputs(cl::Positional, cl::OneOrMore);
static cl::opt<std::string> OutDir("output-dir", cl::Required);
static cl::opt<std::string> Pipeline("passes", cl::init("lto<O2>"));
int main(int argc, char **argv) {
  cl::ParseCommandLineOptions(argc, argv);
  if (Error E = goallc::runFullLTO(Inputs, OutDir, Pipeline, true)) {
    errs() << "goallc-lto: " << toString(std::move(E)) << '\n';
    return 1;
  }
}
