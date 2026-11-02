add_executable(goallc-thinlto-test testdata/thinlto/driver.cpp)
llvm_update_compile_flags(goallc-thinlto-test)
target_include_directories(goallc-thinlto-test PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_compile_features(goallc-thinlto-test PRIVATE cxx_std_17)
target_link_libraries(goallc-thinlto-test PRIVATE GoALLCStatepointsStatic LLVM)
set(THIN_FIXTURES "${CMAKE_CURRENT_SOURCE_DIR}/testdata/thinlto")
set(THIN_OUT "${CMAKE_CURRENT_BINARY_DIR}/thinlto-cross-package")
add_test(NAME thinlto-cross-package
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "'$<TARGET_FILE:goallc-thinlto-test>' --output-dir='${THIN_OUT}' '${THIN_FIXTURES}/caller.ll' '${THIN_FIXTURES}/callee.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${THIN_FIXTURES}/caller.ll' --input-file='${THIN_OUT}/0.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${THIN_FIXTURES}/callee.ll' --input-file='${THIN_OUT}/1.ll'")

set(THIN_DEBUG "${CMAKE_CURRENT_BINARY_DIR}/thinlto-debug")
add_test(NAME thinlto-debug
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "'$<TARGET_FILE:goallc-thinlto-test>' --output-dir='${THIN_DEBUG}' '${THIN_FIXTURES}/debug-caller.ll' '${THIN_FIXTURES}/debug-callee.ll' && '${GOALLC_FILECHECK_EXECUTABLE}' '${THIN_FIXTURES}/debug-caller.ll' --input-file='${THIN_DEBUG}/0.ll'")
set_tests_properties(thinlto-debug PROPERTIES FIXTURES_SETUP thinlto-debug-ir)
foreach(ARCH IN ITEMS x86_64 aarch64)
  add_test(NAME thinlto-debug-${ARCH}
    COMMAND "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      "-mtriple=${ARCH}-unknown-linux-goobj"
      -frame-pointer=all -verify-machineinstrs -filetype=obj
      -o "${THIN_DEBUG}/${ARCH}.o" "${THIN_DEBUG}/0.bc")
  set_tests_properties(thinlto-debug-${ARCH} PROPERTIES
    FIXTURES_REQUIRED thinlto-debug-ir FIXTURES_SETUP thinlto-debug-${ARCH}-obj)
  if(GOALLC_OBJVIEW_EXECUTABLE)
    add_test(NAME thinlto-debug-${ARCH}-object
      COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "'${GOALLC_OBJVIEW_EXECUTABLE}' -json '${THIN_DEBUG}/${ARCH}.o' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=OBJ '${THIN_FIXTURES}/debug-caller.ll'")
    set_tests_properties(thinlto-debug-${ARCH}-object PROPERTIES
      FIXTURES_REQUIRED thinlto-debug-${ARCH}-obj)
  endif()
endforeach()

# Exercise the production buffer ABI with synthetic module identifiers:
# inputs and optimized modules must never be reopened from disk. No diagnostic
# directory may be created when save-temps is disabled.
add_test(NAME thinlto-memory-objects
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "rm -rf '${THIN_OUT}/no-files' && '$<TARGET_FILE:goallc-thinlto-test>' --emit-objects --save-temps=false --output-dir='${THIN_OUT}/no-files' '${THIN_FIXTURES}/debug-caller.ll' '${THIN_FIXTURES}/debug-callee.ll' | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=BUFFER '${THIN_FIXTURES}/debug-caller.ll'")
add_test(NAME thinlto-duplicate-input
  COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
  "'${GOALLC_NOT_EXECUTABLE}' '$<TARGET_FILE:goallc-thinlto-test>' --save-temps=false --output-dir='${THIN_OUT}/duplicate' '${THIN_FIXTURES}/caller.ll' '${THIN_FIXTURES}/caller.ll' 2>&1 | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=DUPLICATE '${THIN_FIXTURES}/caller.ll'")
