// Test-only harness; production calls GoALLCLTO inside cmd/link.
#include "GoALLCLTO.h"
#include "llvm/Support/CommandLine.h"
#include "llvm/Support/raw_ostream.h"
using namespace llvm;
static cl::list<std::string> Inputs(cl::Positional, cl::OneOrMore);
static cl::opt<std::string> OutDir("output-dir", cl::Required);
static cl::opt<bool> SaveIR("save-ir", cl::init(true));
static cl::opt<std::string> Pipeline("passes", cl::init(""));
int main(int argc, char **argv) {
  cl::ParseCommandLineOptions(argc, argv);
  if (Error E =
          goallc::runLTO(Inputs, OutDir,
                         Pipeline.empty() ? "lto<O2>"
                                          : StringRef(Pipeline),
                         SaveIR)) {
    errs() << "goallc-lto: " << toString(std::move(E)) << '\n';
    return 1;
  }
}
