cmake_minimum_required(VERSION 3.20)

project(GoALLCStatepoints LANGUAGES C CXX)

set(GOALLC_OBJVIEW_EXECUTABLE "" CACHE FILEPATH
  "optional objview executable for structured GoObj checks")

find_package(LLVM REQUIRED CONFIG)
if(NOT LLVM_VERSION_MAJOR EQUAL 23)
  message(FATAL_ERROR
    "GoALLCStatepoints requires LLVM 23, found ${LLVM_PACKAGE_VERSION}")
endif()
if(WIN32 OR CYGWIN)
  message(FATAL_ERROR "GoALLCStatepoints does not support Windows plugins")
endif()

list(APPEND CMAKE_MODULE_PATH "${LLVM_CMAKE_DIR}")
include(AddLLVM)

add_definitions(${LLVM_DEFINITIONS})
include_directories(SYSTEM ${LLVM_INCLUDE_DIRS})

add_llvm_pass_plugin(GoALLCStatepoints
  GoALLCPreCodeGen.cpp
  GoALLCStackMapPrinter.cpp
  GoALLCStatepoints.cpp
  GoALLCStatepointPlugin.cpp
)

set_target_properties(GoALLCStatepoints PROPERTIES
  CXX_STANDARD 17
  CXX_STANDARD_REQUIRED YES
  CXX_EXTENSIONS NO
)
if(APPLE)
  target_link_options(GoALLCStatepoints PRIVATE
    "LINKER:-undefined,dynamic_lookup"
  )
endif()

include(CTest)
if(BUILD_TESTING)
  find_package(Python3 REQUIRED COMPONENTS Interpreter)
  find_program(GOALLC_LLC_EXECUTABLE
    NAMES llc
    HINTS "${LLVM_TOOLS_BINARY_DIR}"
    REQUIRED
  )
  find_program(GOALLC_OPT_EXECUTABLE
    NAMES opt
    HINTS "${LLVM_TOOLS_BINARY_DIR}"
    REQUIRED
  )
  add_test(
    NAME GoALLCStatepoints.Load
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-report
      -filetype=null
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/load.ll"
  )
  set_tests_properties(GoALLCStatepoints.Load PROPERTIES
    PASS_REGULAR_EXPRESSION
      "GoALLCStatepoints: ran pre-codegen pipeline for"
  )

  add_test(
    NAME GoALLCStatepoints.Rewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/statepoint.ll"
  )
  set_tests_properties(GoALLCStatepoints.Rewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "p.relocated"
    FAIL_REGULAR_EXPRESSION
      "slot.relocated"
  )

  add_test(
    NAME GoALLCStatepoints.DeferEdge
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/defer-edge.ll"
  )
  set_tests_properties(GoALLCStatepoints.DeferEdge PROPERTIES
    PASS_REGULAR_EXPRESSION
      "callbr void @llvm.go.defer.edge"
    FAIL_REGULAR_EXPRESSION
      "GoALLC statepoints do not yet support invoke or callbr;call goabiinternal void @runtime.deferproc;goallc.defer_result;llvm.memset.inline"
  )

  add_test(
    NAME GoALLCStatepoints.DeferEdgeGoObj
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/defer-edge.goobj"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/defer-edge.ll"
  )

  add_test(
    NAME GoALLCStatepoints.OpenDefer
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-open-defer.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input "${CMAKE_CURRENT_SOURCE_DIR}/testdata/open-defer.ll"
  )

  add_test(
    NAME GoALLCStatepoints.OpenDeferGoObj
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/open-defer.goobj"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/open-defer.ll"
  )

  add_test(
    NAME GoALLCStatepoints.OpenDeferIncompleteFails
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-statepoint-failure.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/open-defer-incomplete.ll"
      --contains
        "GoALLC open-coded defer function is missing frame state metadata"
  )

  add_test(
    NAME GoALLCStatepoints.FunctionMarkerInline
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-function-marker-inline.py"
      --opt "${GOALLC_OPT_EXECUTABLE}"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/function-marker-inline.ll"
  )

  add_test(
    NAME GoALLCStatepoints.PointerAddressObservation
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-pointer-address-observation.py"
      --opt "${GOALLC_OPT_EXECUTABLE}"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/pointer-address-observation.ll"
  )

  add_test(
    NAME GoALLCStatepoints.FrontendMarkersPreserved
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/statepoint.ll"
  )
  set_tests_properties(GoALLCStatepoints.FrontendMarkersPreserved PROPERTIES
    PASS_REGULAR_EXPRESSION
      "go-stack-growth-statepoint"
  )

  add_test(
    NAME GoALLCStatepoints.ZeroLengthPointerArray
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-zero-pointer-array.ll"
  )
  set_tests_properties(GoALLCStatepoints.ZeroLengthPointerArray PROPERTIES
    PASS_REGULAR_EXPRESSION
      "llvm.experimental.gc.statepoint"
    FAIL_REGULAR_EXPRESSION
      "pointer-aligned fixed alloca layouts"
  )

  add_test(
    NAME GoALLCStatepoints.GCLeafMarkers
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/gc-leaf-markers.ll"
  )
  set_tests_properties(GoALLCStatepoints.GCLeafMarkers PROPERTIES
    PASS_REGULAR_EXPRESSION
      "callsite_leaf"
    FAIL_REGULAR_EXPRESSION
      "gc\\.statepoint"
  )

  add_test(
    NAME GoALLCStatepoints.NestParamAttrRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/nest-param-attr.ll"
  )
  set_tests_properties(GoALLCStatepoints.NestParamAttrRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "ptr nest %context"
  )

  add_test(
    NAME GoALLCStatepoints.SupportedParamAttrsRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/supported-param-attrs.ll"
  )
  set_tests_properties(GoALLCStatepoints.SupportedParamAttrsRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "ptr noundef nonnull readnone align 8 %argument"
  )

  add_test(
    NAME GoALLCStatepoints.SupportedParamAttrsCodegen
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/supported-param-attrs.goobj"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/supported-param-attrs.ll"
  )

  add_test(
    NAME GoALLCStatepoints.CallOnlyPointersNotLive
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-call-only-liveness.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input "${CMAKE_CURRENT_SOURCE_DIR}/testdata/indirect-callee.ll"
  )

  foreach(GOALLC_CALLIND_TARGET IN ITEMS X86 AArch64)
    if(GOALLC_CALLIND_TARGET STREQUAL "X86")
      set(GOALLC_CALLIND_TRIPLE "x86_64-unknown-linux-goobj")
      set(GOALLC_DIRECT_CALL_RELOC 7)
    else()
      set(GOALLC_CALLIND_TRIPLE "aarch64-unknown-linux-goobj")
      set(GOALLC_DIRECT_CALL_RELOC 9)
    endif()
    add_test(
      NAME GoALLCStatepoints.${GOALLC_CALLIND_TARGET}IndirectCallRelocation
      COMMAND
        "${Python3_EXECUTABLE}"
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-call-relocations.py"
        --llc "${GOALLC_LLC_EXECUTABLE}"
        --plugin "$<TARGET_FILE:GoALLCStatepoints>"
        --input "${CMAKE_CURRENT_SOURCE_DIR}/testdata/indirect-callee.ll"
        --triple "${GOALLC_CALLIND_TRIPLE}"
        --direct-reloc "${GOALLC_DIRECT_CALL_RELOC}"
        --output
          "${CMAKE_CURRENT_BINARY_DIR}/${GOALLC_CALLIND_TARGET}-indirect.goobj"
    )
  endforeach()

  add_test(
    NAME GoALLCStatepoints.AllocaDerivedRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-derived.ll"
  )
  set_tests_properties(GoALLCStatepoints.AllocaDerivedRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "selected.relocated"
  )

  add_test(
    NAME GoALLCStatepoints.DerivedPointerRematerialization
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-derived-pointer-rematerialization.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/derived-pointer-rematerialization.ll"
  )

  add_test(
    NAME GoALLCStatepoints.GoObj
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/statepoint.goobj"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/statepoint.ll"
  )

  add_test(
    NAME GoALLCStatepoints.AllocaDerivedGoObj
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/alloca-derived.goobj"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-derived.ll"
  )

  add_test(
    NAME GoALLCStatepoints.AArch64FrameGoObj
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/aarch64-frame.goobj"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aarch64-frame.ll"
  )

  add_test(
    NAME GoALLCStatepoints.X86ABI0EntryArgsGoObj
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/x86-abi0-entry-args.goobj"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/x86-abi0-entry-args.ll"
  )

  add_test(
    NAME GoALLCStatepoints.MultipleCallsRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/multiple-calls.ll"
  )
  set_tests_properties(GoALLCStatepoints.MultipleCallsRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "q.relocated2"
  )

  add_test(
    NAME GoALLCStatepoints.MultipleCallsGoObj
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/multiple-calls.goobj"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/multiple-calls.ll"
  )

  add_test(
    NAME GoALLCStatepoints.AggregateCallResultGoObj
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/aggregate-call-result.goobj"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-call-result-goobj.ll"
  )

  if(GOALLC_OBJVIEW_EXECUTABLE)
    if(NOT EXISTS "${GOALLC_OBJVIEW_EXECUTABLE}")
      message(FATAL_ERROR
        "GOALLC_OBJVIEW_EXECUTABLE does not exist: "
        "${GOALLC_OBJVIEW_EXECUTABLE}")
    endif()
    add_test(
      NAME GoALLCStatepoints.DeferEdgeObjView
      COMMAND
        "${Python3_EXECUTABLE}"
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-defer-edge.py"
        --objview "${GOALLC_OBJVIEW_EXECUTABLE}"
        --object "${CMAKE_CURRENT_BINARY_DIR}/defer-edge.goobj"
    )
    set_tests_properties(GoALLCStatepoints.DeferEdgeObjView PROPERTIES
      DEPENDS GoALLCStatepoints.DeferEdgeGoObj
    )

    add_test(
      NAME GoALLCStatepoints.OpenDeferObjView
      COMMAND
        "${Python3_EXECUTABLE}"
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-open-defer.py"
        --llc "${GOALLC_LLC_EXECUTABLE}"
        --plugin "$<TARGET_FILE:GoALLCStatepoints>"
        --input "${CMAKE_CURRENT_SOURCE_DIR}/testdata/open-defer.ll"
        --objview "${GOALLC_OBJVIEW_EXECUTABLE}"
    )
    set_tests_properties(GoALLCStatepoints.OpenDeferObjView PROPERTIES
      DEPENDS GoALLCStatepoints.OpenDeferGoObj
    )

    add_test(
      NAME GoALLCStatepoints.MultipleCallsObjView
      COMMAND
        "${Python3_EXECUTABLE}"
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-multiple-calls.py"
        --objview "${GOALLC_OBJVIEW_EXECUTABLE}"
        --object "${CMAKE_CURRENT_BINARY_DIR}/multiple-calls.goobj"
    )
    set_tests_properties(GoALLCStatepoints.MultipleCallsObjView PROPERTIES
      DEPENDS GoALLCStatepoints.MultipleCallsGoObj
    )

    add_test(
      NAME GoALLCStatepoints.AArch64FrameObjView
      COMMAND
        "${Python3_EXECUTABLE}"
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-aarch64-frame.py"
        --objview "${GOALLC_OBJVIEW_EXECUTABLE}"
        --object "${CMAKE_CURRENT_BINARY_DIR}/aarch64-frame.goobj"
    )
    set_tests_properties(GoALLCStatepoints.AArch64FrameObjView PROPERTIES
      DEPENDS GoALLCStatepoints.AArch64FrameGoObj
    )

    add_test(
      NAME GoALLCStatepoints.X86ABI0EntryArgsObjView
      COMMAND
        "${Python3_EXECUTABLE}"
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-x86-abi0-entry-args.py"
        --objview "${GOALLC_OBJVIEW_EXECUTABLE}"
        --object "${CMAKE_CURRENT_BINARY_DIR}/x86-abi0-entry-args.goobj"
    )
    set_tests_properties(GoALLCStatepoints.X86ABI0EntryArgsObjView PROPERTIES
      DEPENDS GoALLCStatepoints.X86ABI0EntryArgsGoObj
    )

    add_test(
      NAME GoALLCStatepoints.ConditionalRelocationObjView
      COMMAND
        "${Python3_EXECUTABLE}"
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-conditional-relocation.py"
        --objview "${GOALLC_OBJVIEW_EXECUTABLE}"
        --object "${CMAKE_CURRENT_BINARY_DIR}/conditional-relocation.goobj"
    )
    set_tests_properties(GoALLCStatepoints.ConditionalRelocationObjView PROPERTIES
      DEPENDS GoALLCStatepoints.ConditionalRelocationGoObj
    )

    add_test(
      NAME GoALLCStatepoints.AggregateCallResultObjView
      COMMAND
        "${Python3_EXECUTABLE}"
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-aggregate-call-result.py"
        --objview "${GOALLC_OBJVIEW_EXECUTABLE}"
        --object "${CMAKE_CURRENT_BINARY_DIR}/aggregate-call-result.goobj"
    )
    set_tests_properties(GoALLCStatepoints.AggregateCallResultObjView PROPERTIES
      DEPENDS GoALLCStatepoints.AggregateCallResultGoObj
    )
  endif()

  add_test(
    NAME GoALLCStatepoints.LiveAggregateRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/live-aggregate.ll"
  )
  set_tests_properties(GoALLCStatepoints.LiveAggregateRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "value\\.leaf\\.0\\.relocated"
  )

  add_test(
    NAME GoALLCStatepoints.AggregateScalarization
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-aggregate-scalarization.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --opt "${GOALLC_OPT_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-scalarization.ll"
  )

  add_test(
    NAME GoALLCStatepoints.AggregateFixedVectorRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-fixed-vector.ll"
  )
  set_tests_properties(GoALLCStatepoints.AggregateFixedVectorRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "call coldcc <2 x ptr> @llvm\\.experimental\\.gc\\.relocate\\.v2p0"
  )

  add_test(
    NAME GoALLCStatepoints.AggregateScalableVectorFails
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-statepoint-failure.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-scalable-vector-unsupported.ll"
      --contains
      "do not support scalable vectors inside live pointer aggregates"
  )

  set(GOALLC_ALLOCA_POINTER_ROOTS_EXTRA_ARGS)
  if(GOALLC_OBJVIEW_EXECUTABLE)
    list(APPEND GOALLC_ALLOCA_POINTER_ROOTS_EXTRA_ARGS
      --objview "${GOALLC_OBJVIEW_EXECUTABLE}")
  endif()
  add_test(
    NAME GoALLCStatepoints.AllocaPointerRoots
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-alloca-pointer-roots.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --opt "${GOALLC_OPT_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll"
      ${GOALLC_ALLOCA_POINTER_ROOTS_EXTRA_ARGS}
  )

  add_test(
    NAME GoALLCStatepoints.AllocaPointerRootsAArch64
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-alloca-pointer-roots.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --opt "${GOALLC_OPT_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --mtriple aarch64-unknown-linux-goobj
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll"
      ${GOALLC_ALLOCA_POINTER_ROOTS_EXTRA_ARGS}
  )

  add_test(
    NAME GoALLCStatepoints.MalformedAllocaPointerMapsFail
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-alloca-ptrmap-malformed.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
  )

  add_test(
    NAME GoALLCStatepoints.PointerAllocaSelectSupported
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=null
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-select-unsupported.ll"
  )

  add_test(
    NAME GoALLCStatepoints.NonEntryPointerAllocaFails
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-statepoint-failure.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-nonentry-unsupported.ll"
      --contains
      "require a single fixed entry-block pointer-containing alloca"
  )

  add_test(
    NAME GoALLCStatepoints.DynamicPointerAllocaFails
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-statepoint-failure.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-dynamic-unsupported.ll"
      --contains
      "require a single fixed entry-block pointer-containing alloca"
  )

  add_test(
    NAME GoALLCStatepoints.RealignedPointerAllocaFails
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-statepoint-failure.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-realigned-unsupported.ll"
      --contains
      "do not support realigned pointer-containing allocas"
  )

  add_test(
    NAME GoALLCStatepoints.PointerVectorAllocaFails
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-statepoint-failure.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-vector-unsupported.ll"
      --contains
      "do not support pointer vectors in allocas"
  )

  set(GOALLC_POINTER_ALLOCA_LIFETIME_EXTRA_ARGS)
  if(GOALLC_OBJVIEW_EXECUTABLE)
    list(APPEND GOALLC_POINTER_ALLOCA_LIFETIME_EXTRA_ARGS
      --objview "${GOALLC_OBJVIEW_EXECUTABLE}")
  endif()
  add_test(
    NAME GoALLCStatepoints.PointerAllocaLifetimeSupported
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-alloca-pointer-lifetime.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-lifetime-unsupported.ll"
      ${GOALLC_POINTER_ALLOCA_LIFETIME_EXTRA_ARGS}
  )

  add_test(
    NAME GoALLCStatepoints.PathLocalPointerAllocaLifetime
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-lifetime-ambiguous.ll"
  )
  set_tests_properties(
    GoALLCStatepoints.PathLocalPointerAllocaLifetime PROPERTIES
    PASS_REGULAR_EXPRESSION "1095520067")

  add_test(
    NAME GoALLCStatepoints.VolatilePointerAllocaSupported
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=null
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-volatile-unsupported.ll"
  )

  add_test(
    NAME GoALLCStatepoints.AggregateConditionalRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-conditional-stage-b.ll"
  )
  set_tests_properties(GoALLCStatepoints.AggregateConditionalRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "value\\.leaf\\.0\\.relocated\\.merge.* = phi ptr"
  )

  add_test(
    NAME GoALLCStatepoints.AggregateConditionalStableID
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-conditional-stage-b.ll"
  )
  set_tests_properties(GoALLCStatepoints.AggregateConditionalStableID PROPERTIES
    PASS_REGULAR_EXPRESSION
      "i64 -978360670708369630"
  )

  add_test(
    NAME GoALLCStatepoints.AggregateLoopRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-loop-stage-b.ll"
  )
  set_tests_properties(GoALLCStatepoints.AggregateLoopRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "value\\.leaf\\.0\\.relocated\\.merge.* = phi ptr"
  )

  add_test(
    NAME GoALLCStatepoints.AggregateCFGRewrite
    COMMAND
      "${Python3_EXECUTABLE}"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/check-aggregate-cfg.py"
      --llc "${GOALLC_LLC_EXECUTABLE}"
      --opt "${GOALLC_OPT_EXECUTABLE}"
      --plugin "$<TARGET_FILE:GoALLCStatepoints>"
      --input
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-cfg.ll"
  )

  add_test(
    NAME GoALLCStatepoints.ConditionalRelocationRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/conditional-relocation.ll"
  )
  set_tests_properties(GoALLCStatepoints.ConditionalRelocationRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "p\\.relocated\\.merge.* = phi ptr"
  )

  add_test(
    NAME GoALLCStatepoints.ConditionalPhiEdgeUseRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/conditional-relocation.ll"
  )
  set_tests_properties(GoALLCStatepoints.ConditionalPhiEdgeUseRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "selected = phi ptr \\[ %p\\.relocated, %call \\]"
  )

  add_test(
    NAME GoALLCStatepoints.ConditionalRelocationStableID
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/conditional-relocation.ll"
  )
  set_tests_properties(GoALLCStatepoints.ConditionalRelocationStableID PROPERTIES
    PASS_REGULAR_EXPRESSION
      "i64 -6495201133029790441"
  )

  add_test(
    NAME GoALLCStatepoints.ConditionalRelocationGoObj
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/conditional-relocation.goobj"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/conditional-relocation.ll"
  )

  add_test(
    NAME GoALLCStatepoints.BranchSafepointsRelocationRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/branch-safepoints-relocation.ll"
  )
  set_tests_properties(GoALLCStatepoints.BranchSafepointsRelocationRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "p\\.relocated\\.merge.* = phi ptr"
  )

  add_test(
    NAME GoALLCStatepoints.SequentialConditionalRelocationRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/sequential-conditional-relocation.ll"
  )
  set_tests_properties(GoALLCStatepoints.SequentialConditionalRelocationRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "p\\.relocated\\.merge.* = phi ptr"
  )

  add_test(
    NAME GoALLCStatepoints.SequentialConditionalLiveSetRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/sequential-conditional-relocation.ll"
  )
  set_tests_properties(GoALLCStatepoints.SequentialConditionalLiveSetRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "\"gc-live\"\\(ptr %p\\.relocated\\.merge"
  )

  add_test(
    NAME GoALLCStatepoints.SameBlockRelocationChainRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/statepoint.ll"
  )
  set_tests_properties(GoALLCStatepoints.SameBlockRelocationChainRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "\"gc-live\"\\(ptr %p\\.relocated2"
  )

  add_test(
    NAME GoALLCStatepoints.OutOfLayoutCallResultRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/statepoint.ll"
  )
  set_tests_properties(GoALLCStatepoints.OutOfLayoutCallResultRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "define goabiinternal ptr @out_of_layout_call_result"
  )

  add_test(
    NAME GoALLCStatepoints.LoopRelocationRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/loop-relocation.ll"
  )
  set_tests_properties(GoALLCStatepoints.LoopRelocationRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "p\\.relocated\\.merge\\.1 = phi ptr"
  )

  add_test(
    NAME GoALLCStatepoints.IrreducibleRelocationRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/irreducible-relocation.ll"
  )
  set_tests_properties(GoALLCStatepoints.IrreducibleRelocationRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "p\\.relocated\\.merge.* = phi ptr"
  )

  add_test(
    NAME GoALLCStatepoints.InvokeFails
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/unsupported-invoke.ll"
  )
  set_tests_properties(GoALLCStatepoints.InvokeFails PROPERTIES
    WILL_FAIL TRUE
  )

  add_test(
    NAME GoALLCStatepoints.ParamAttrFails
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/unsupported-param-attr.ll"
  )
  set_tests_properties(GoALLCStatepoints.ParamAttrFails PROPERTIES
    WILL_FAIL TRUE
  )

  add_test(
    NAME GoALLCStatepoints.InvalidGCLeafDefinitionFails
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/invalid-gc-leaf-definition.ll"
  )
  set_tests_properties(GoALLCStatepoints.InvalidGCLeafDefinitionFails PROPERTIES
    WILL_FAIL TRUE
  )
endif()
