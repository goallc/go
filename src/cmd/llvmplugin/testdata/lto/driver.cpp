// Test-only harness; production calls GoALLCLTO inside cmd/link.
#include "GoALLCLTO.h"
#include "llvm/Support/CommandLine.h"
#include "llvm/Support/raw_ostream.h"
using namespace llvm;
static cl::list<std::string> Inputs(cl::Positional, cl::OneOrMore);
static cl::opt<std::string> OutDir("output-dir", cl::Required);
static cl::opt<bool> Thin("thin");
static cl::opt<bool> Single("single");
static cl::opt<std::string> Pipeline("passes", cl::init(""));
int main(int argc, char **argv) {
  cl::ParseCommandLineOptions(argc, argv);
  if (Error E =
          goallc::runLTO(Inputs, OutDir,
                         Pipeline.empty() ? (Thin ? "thinlto<O2>" : "lto<O2>")
                                          : StringRef(Pipeline),
                         true, Thin, Single)) {
    errs() << "goallc-lto: " << toString(std::move(E)) << '\n';
    return 1;
  }
}
