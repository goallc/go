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
  GoALLCStackMapPrinter.cpp
  GoALLCInlineAnchors.cpp
  GoALLCPreCodeGen.cpp
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
  find_program(GOALLC_FILECHECK_EXECUTABLE
    NAMES FileCheck
    HINTS "${LLVM_TOOLS_BINARY_DIR}"
    REQUIRED
  )
  find_program(GOALLC_NOT_EXECUTABLE
    NAMES not
    HINTS "${LLVM_TOOLS_BINARY_DIR}"
    REQUIRED
  )
  find_program(GOALLC_BASH_EXECUTABLE NAMES bash REQUIRED)

  # CTest does not model pipelines directly. Keep the shell glue here and put
  # every assertion in the LLVM input next to the IR that it describes.
  function(goallc_add_filecheck_test NAME CHECK_FILE CHECK_PREFIXES)
    set(GOALLC_COMMAND_TEXT "")
    foreach(GOALLC_COMMAND_ARG IN LISTS ARGN)
      string(REPLACE "'" "'\"'\"'" GOALLC_QUOTED_ARG
        "${GOALLC_COMMAND_ARG}")
      string(APPEND GOALLC_COMMAND_TEXT " '${GOALLC_QUOTED_ARG}'")
    endforeach()
    string(REPLACE "'" "'\"'\"'" GOALLC_QUOTED_CHECK_FILE
      "${CHECK_FILE}")
    string(APPEND GOALLC_COMMAND_TEXT
      " 2>&1 | '${GOALLC_FILECHECK_EXECUTABLE}'"
      " --check-prefixes='${CHECK_PREFIXES}' '${GOALLC_QUOTED_CHECK_FILE}'")
    add_test(
      NAME "${NAME}"
      COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
        "${GOALLC_COMMAND_TEXT}"
    )
  endfunction()

  function(goallc_add_ir_filecheck_test NAME INPUT CHECK_PREFIXES)
    goallc_add_filecheck_test(
      "${NAME}" "${INPUT}" "${CHECK_PREFIXES}"
      "${GOALLC_LLC_EXECUTABLE}"
      ${ARGN}
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${INPUT}"
    )
  endfunction()

  function(goallc_add_failure_filecheck_test NAME INPUT CHECK_PREFIXES)
    goallc_add_filecheck_test(
      "${NAME}" "${INPUT}" "${CHECK_PREFIXES}"
      "${GOALLC_NOT_EXECUTABLE}"
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${INPUT}"
    )
  endfunction()
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

  goallc_add_ir_filecheck_test(
    GoALLCStatepoints.OpenDefer
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/open-defer.ll"
    OPEN-DEFER
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

  goallc_add_failure_filecheck_test(
    GoALLCStatepoints.OpenDeferIncompleteFails
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/open-defer-incomplete.ll"
    ERROR
  )

  goallc_add_filecheck_test(
    GoALLCStatepoints.FunctionMarkerInlineOpt
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/function-marker-inline.ll"
    INLINE-OPT
    "${GOALLC_OPT_EXECUTABLE}"
    -S "-passes=default<O2>"
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/function-marker-inline.ll"
    -o -
  )
  set(GOALLC_FUNCTION_MARKER_PIPELINE
    "'${GOALLC_OPT_EXECUTABLE}' -S '-passes=default<O2>'"
    " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/function-marker-inline.ll' -o -"
    " | '${GOALLC_LLC_EXECUTABLE}'"
    " '-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>'"
    " -goallc-pass-plugin-emit-ir -filetype=null -o - -"
    " 2>&1"
    " | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=INLINE-REWRITE"
    " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/function-marker-inline.ll'")
  string(JOIN "" GOALLC_FUNCTION_MARKER_PIPELINE
    ${GOALLC_FUNCTION_MARKER_PIPELINE})
  add_test(
    NAME GoALLCStatepoints.FunctionMarkerInlineRewrite
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "${GOALLC_FUNCTION_MARKER_PIPELINE}"
  )

  set(GOALLC_POINTER_ADDRESS_PIPELINE
    "'${GOALLC_OPT_EXECUTABLE}' -S '-passes=default<O2>' -o -"
    " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/pointer-address-observation.ll'"
    " | '${GOALLC_LLC_EXECUTABLE}'"
    " '-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>'"
    " -goallc-pass-plugin-emit-ir -filetype=null -o - - 2>&1"
    " | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=REWRITE"
    " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/pointer-address-observation.ll'")
  string(JOIN "" GOALLC_POINTER_ADDRESS_PIPELINE
    ${GOALLC_POINTER_ADDRESS_PIPELINE})
  add_test(
    NAME GoALLCStatepoints.PointerAddressObservation
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "${GOALLC_POINTER_ADDRESS_PIPELINE}"
  )
  goallc_add_filecheck_test(
    GoALLCStatepoints.PointerAddressObservationO2
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/pointer-address-observation.ll"
    O2
    "${GOALLC_OPT_EXECUTABLE}" -S "-passes=default<O2>" -o -
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
      "gc \"goallc\""
  )

  foreach(GOALLC_DEBUG_TARGET IN ITEMS X86 AArch64)
    if(GOALLC_DEBUG_TARGET STREQUAL "X86")
      set(GOALLC_DEBUG_TRIPLE x86_64-unknown-linux-goobj)
      set(GOALLC_DEBUG_CHECK_PREFIX DEBUG-X86)
    else()
      set(GOALLC_DEBUG_TRIPLE aarch64-unknown-linux-goobj)
      set(GOALLC_DEBUG_CHECK_PREFIX DEBUG-AARCH64)
    endif()
    add_test(
      NAME GoALLCStatepoints.DebugInline${GOALLC_DEBUG_TARGET}GoObj
      COMMAND "${GOALLC_LLC_EXECUTABLE}"
        "-mtriple=${GOALLC_DEBUG_TRIPLE}"
        "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
        -verify-machineinstrs -filetype=obj
        -o "${CMAKE_CURRENT_BINARY_DIR}/debug-inline-${GOALLC_DEBUG_TARGET}.goobj"
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/debug-inline.ll"
    )
  endforeach()

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
    NAME GoALLCStatepoints.PreallocatedParamAttrRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/supported-param-attrs.ll"
  )
  set_tests_properties(GoALLCStatepoints.PreallocatedParamAttrRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "ptr preallocated.*5 x ptr.* align 8 %home"
  )

  add_test(
    NAME GoALLCStatepoints.GoRetResultAttrRewrite
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -goallc-pass-plugin-emit-ir
      -filetype=null
      -o -
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/supported-param-attrs.ll"
  )
  set_tests_properties(GoALLCStatepoints.GoRetResultAttrRewrite PROPERTIES
    PASS_REGULAR_EXPRESSION
      "ptr goret.*5 x ptr.* align 8.*goretindex.*0.* %stack_result.address"
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

  goallc_add_ir_filecheck_test(
    GoALLCStatepoints.CallOnlyPointersNotLive
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/indirect-callee.ll"
    IR
  )

  foreach(GOALLC_CALLIND_TARGET IN ITEMS X86 AArch64)
    if(GOALLC_CALLIND_TARGET STREQUAL "X86")
      set(GOALLC_CALLIND_TRIPLE "x86_64-unknown-linux-goobj")
    else()
      set(GOALLC_CALLIND_TRIPLE "aarch64-unknown-linux-goobj")
    endif()
    add_test(
      NAME GoALLCStatepoints.${GOALLC_CALLIND_TARGET}IndirectCallGoObj
      COMMAND
        "${GOALLC_LLC_EXECUTABLE}"
        "-mtriple=${GOALLC_CALLIND_TRIPLE}"
        "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
        -verify-machineinstrs
        -filetype=obj
        -o
          "${CMAKE_CURRENT_BINARY_DIR}/${GOALLC_CALLIND_TARGET}-indirect.goobj"
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/indirect-callee.ll"
    )
    if(GOALLC_OBJVIEW_EXECUTABLE)
      goallc_add_filecheck_test(
        GoALLCStatepoints.${GOALLC_CALLIND_TARGET}IndirectCallRelocation
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/indirect-callee.ll"
        "${GOALLC_CALLIND_TARGET}-OBJVIEW"
        "${GOALLC_OBJVIEW_EXECUTABLE}" -format=text
        "${CMAKE_CURRENT_BINARY_DIR}/${GOALLC_CALLIND_TARGET}-indirect.goobj"
      )
      set_tests_properties(
        GoALLCStatepoints.${GOALLC_CALLIND_TARGET}IndirectCallRelocation
        PROPERTIES
        DEPENDS GoALLCStatepoints.${GOALLC_CALLIND_TARGET}IndirectCallGoObj
      )
    endif()
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

  goallc_add_ir_filecheck_test(
    GoALLCStatepoints.DerivedPointerRematerialization
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/derived-pointer-rematerialization.ll"
    IR
    -verify-machineinstrs
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
    foreach(GOALLC_DEBUG_TARGET IN ITEMS X86 AArch64)
      if(GOALLC_DEBUG_TARGET STREQUAL "X86")
        set(GOALLC_DEBUG_CHECK_PREFIX DEBUG-X86)
      else()
        set(GOALLC_DEBUG_CHECK_PREFIX DEBUG-AARCH64)
      endif()
      goallc_add_filecheck_test(
        GoALLCStatepoints.DebugInline${GOALLC_DEBUG_TARGET}ObjView
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/debug-inline.ll"
        "DEBUG,${GOALLC_DEBUG_CHECK_PREFIX}"
        "${GOALLC_OBJVIEW_EXECUTABLE}" -json
        "${CMAKE_CURRENT_BINARY_DIR}/debug-inline-${GOALLC_DEBUG_TARGET}.goobj"
      )
      set_tests_properties(
        GoALLCStatepoints.DebugInline${GOALLC_DEBUG_TARGET}ObjView PROPERTIES
        DEPENDS GoALLCStatepoints.DebugInline${GOALLC_DEBUG_TARGET}GoObj
      )
    endforeach()
    goallc_add_filecheck_test(
      GoALLCStatepoints.GoObjStackMaps
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/statepoint.ll"
      OBJVIEW-TEXT
      "${GOALLC_OBJVIEW_EXECUTABLE}" -format=text
      "${CMAKE_CURRENT_BINARY_DIR}/statepoint.goobj"
    )
    set_tests_properties(GoALLCStatepoints.GoObjStackMaps PROPERTIES
      DEPENDS GoALLCStatepoints.GoObj
    )

    goallc_add_filecheck_test(
      GoALLCStatepoints.DeferEdgeObjView
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/defer-edge.ll"
      OBJVIEW-TEXT
      "${GOALLC_OBJVIEW_EXECUTABLE}" -format=text
      "${CMAKE_CURRENT_BINARY_DIR}/defer-edge.goobj"
    )
    set_tests_properties(GoALLCStatepoints.DeferEdgeObjView PROPERTIES
      DEPENDS GoALLCStatepoints.DeferEdgeGoObj
    )
    goallc_add_filecheck_test(
      GoALLCStatepoints.DeferEdgeObjViewJSON
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/defer-edge.ll"
      OBJVIEW-JSON
      "${GOALLC_OBJVIEW_EXECUTABLE}" -json
      "${CMAKE_CURRENT_BINARY_DIR}/defer-edge.goobj"
    )
    set_tests_properties(GoALLCStatepoints.DeferEdgeObjViewJSON PROPERTIES
      DEPENDS GoALLCStatepoints.DeferEdgeGoObj
    )

    goallc_add_filecheck_test(
      GoALLCStatepoints.OpenDeferObjView
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/open-defer.ll"
      OBJVIEW
      "${GOALLC_OBJVIEW_EXECUTABLE}" -json
      "${CMAKE_CURRENT_BINARY_DIR}/open-defer.goobj"
    )
    set_tests_properties(GoALLCStatepoints.OpenDeferObjView PROPERTIES
      DEPENDS GoALLCStatepoints.OpenDeferGoObj
    )

    goallc_add_filecheck_test(
      GoALLCStatepoints.MultipleCallsObjView
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/multiple-calls.ll"
      OBJVIEW
      "${GOALLC_OBJVIEW_EXECUTABLE}" -json
      "${CMAKE_CURRENT_BINARY_DIR}/multiple-calls.goobj"
    )
    set_tests_properties(GoALLCStatepoints.MultipleCallsObjView PROPERTIES
      DEPENDS GoALLCStatepoints.MultipleCallsGoObj
    )

    goallc_add_filecheck_test(
      GoALLCStatepoints.AArch64FrameObjView
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aarch64-frame.ll"
      OBJVIEW
      "${GOALLC_OBJVIEW_EXECUTABLE}" -json
      "${CMAKE_CURRENT_BINARY_DIR}/aarch64-frame.goobj"
    )
    set_tests_properties(GoALLCStatepoints.AArch64FrameObjView PROPERTIES
      DEPENDS GoALLCStatepoints.AArch64FrameGoObj
    )
    goallc_add_filecheck_test(
      GoALLCStatepoints.AArch64FrameObjViewText
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aarch64-frame.ll"
      "ASM,FRAME-TEXT"
      "${GOALLC_OBJVIEW_EXECUTABLE}" -format=text
      "${CMAKE_CURRENT_BINARY_DIR}/aarch64-frame.goobj"
    )
    set_tests_properties(GoALLCStatepoints.AArch64FrameObjViewText PROPERTIES
      DEPENDS GoALLCStatepoints.AArch64FrameGoObj
    )

    goallc_add_filecheck_test(
      GoALLCStatepoints.ConditionalRelocationObjView
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/conditional-relocation.ll"
      OBJVIEW
      "${GOALLC_OBJVIEW_EXECUTABLE}" -json
      "${CMAKE_CURRENT_BINARY_DIR}/conditional-relocation.goobj"
    )
    set_tests_properties(GoALLCStatepoints.ConditionalRelocationObjView PROPERTIES
      DEPENDS GoALLCStatepoints.ConditionalRelocationGoObj
    )

    goallc_add_filecheck_test(
      GoALLCStatepoints.AggregateCallResultObjView
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-call-result-goobj.ll"
      OBJVIEW
      "${GOALLC_OBJVIEW_EXECUTABLE}" -json
      "${CMAKE_CURRENT_BINARY_DIR}/aggregate-call-result.goobj"
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

  set(GOALLC_AGGREGATE_SCALARIZATION_PIPELINE
    "'${GOALLC_LLC_EXECUTABLE}'"
    " '-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>'"
    " -goallc-pass-plugin-emit-ir -filetype=null -o -"
    " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-scalarization.ll'"
    " 2>&1"
    " | '${GOALLC_OPT_EXECUTABLE}'"
    " '-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>'"
    " -S -passes=verify -o - -"
    " | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=IR"
    " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-scalarization.ll'")
  string(JOIN "" GOALLC_AGGREGATE_SCALARIZATION_PIPELINE
    ${GOALLC_AGGREGATE_SCALARIZATION_PIPELINE})
  add_test(
    NAME GoALLCStatepoints.AggregateScalarization
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "${GOALLC_AGGREGATE_SCALARIZATION_PIPELINE}"
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

  goallc_add_failure_filecheck_test(
    GoALLCStatepoints.AggregateScalableVectorFails
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-scalable-vector-unsupported.ll"
    ERROR
  )

  goallc_add_ir_filecheck_test(
    GoALLCStatepoints.AllocaPointerRoots
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll"
    IR
  )
  goallc_add_ir_filecheck_test(
    GoALLCStatepoints.AllocaPointerRootsAArch64
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll"
    IR
    -mtriple=aarch64-unknown-linux-goobj
    -frame-pointer=all
  )
  goallc_add_filecheck_test(
    GoALLCStatepoints.AllocaPointerRootsMIR
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll"
    MIR
    "${GOALLC_LLC_EXECUTABLE}"
    "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
    -verify-machineinstrs
    -stop-after=finalize-isel
    -o -
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll"
  )
  goallc_add_filecheck_test(
    GoALLCStatepoints.AllocaPointerRootsMIRStatepointCount
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll"
    MIR-ALL
    "${GOALLC_LLC_EXECUTABLE}"
    "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
    -verify-machineinstrs
    -stop-after=finalize-isel
    -o -
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll"
  )
  set(GOALLC_ALLOCA_O2_PIPELINE
    "'${GOALLC_OPT_EXECUTABLE}' '-passes=default<O2>,verify' -S -o -"
    " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll'"
    " | '${GOALLC_LLC_EXECUTABLE}'"
    " '-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>'"
    " -goallc-pass-plugin-emit-ir -filetype=null -o - - 2>&1"
    " | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=O2"
    " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll'")
  string(JOIN "" GOALLC_ALLOCA_O2_PIPELINE ${GOALLC_ALLOCA_O2_PIPELINE})
  add_test(
    NAME GoALLCStatepoints.AllocaPointerRootsO2
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "${GOALLC_ALLOCA_O2_PIPELINE}"
  )
  foreach(GOALLC_ALLOCA_TARGET IN ITEMS X86 AArch64)
    if(GOALLC_ALLOCA_TARGET STREQUAL "X86")
      set(GOALLC_ALLOCA_TRIPLE x86_64-unknown-linux-goobj)
      set(GOALLC_ALLOCA_FRAME_ARGS)
    else()
      set(GOALLC_ALLOCA_TRIPLE aarch64-unknown-linux-goobj)
      set(GOALLC_ALLOCA_FRAME_ARGS -frame-pointer=all)
    endif()
    add_test(
      NAME GoALLCStatepoints.AllocaPointerRoots${GOALLC_ALLOCA_TARGET}GoObj
      COMMAND "${GOALLC_LLC_EXECUTABLE}"
        "-mtriple=${GOALLC_ALLOCA_TRIPLE}"
        ${GOALLC_ALLOCA_FRAME_ARGS}
        "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
        -verify-machineinstrs -filetype=obj
        -o "${CMAKE_CURRENT_BINARY_DIR}/alloca-pointer-roots-${GOALLC_ALLOCA_TARGET}.goobj"
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll"
    )
    if(GOALLC_OBJVIEW_EXECUTABLE)
      goallc_add_filecheck_test(
        GoALLCStatepoints.AllocaPointerRoots${GOALLC_ALLOCA_TARGET}ObjView
        "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-roots.ll"
        OBJVIEW
        "${GOALLC_OBJVIEW_EXECUTABLE}" -json
        "${CMAKE_CURRENT_BINARY_DIR}/alloca-pointer-roots-${GOALLC_ALLOCA_TARGET}.goobj"
      )
      set_tests_properties(
        GoALLCStatepoints.AllocaPointerRoots${GOALLC_ALLOCA_TARGET}ObjView
        PROPERTIES
        DEPENDS GoALLCStatepoints.AllocaPointerRoots${GOALLC_ALLOCA_TARGET}GoObj
      )
    endif()
  endforeach()

  foreach(GOALLC_MALFORMED_PTRMAP_CASE IN ITEMS
      bad-kind truncated bad-length duplicate overlap padding non-direct)
    set(GOALLC_MALFORMED_PTRMAP_INPUT
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-ptrmap-malformed-${GOALLC_MALFORMED_PTRMAP_CASE}.ll")
    goallc_add_filecheck_test(
      "GoALLCStatepoints.MalformedAllocaPointerMap.${GOALLC_MALFORMED_PTRMAP_CASE}"
      "${GOALLC_MALFORMED_PTRMAP_INPUT}"
      ERROR
      "${GOALLC_NOT_EXECUTABLE}"
      --crash
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=obj
      -o /dev/null
      "${GOALLC_MALFORMED_PTRMAP_INPUT}"
    )
  endforeach()

  add_test(
    NAME GoALLCStatepoints.PointerAllocaSelectSupported
    COMMAND
      "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs
      -filetype=null
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-select-unsupported.ll"
  )

  goallc_add_failure_filecheck_test(
    GoALLCStatepoints.NonEntryPointerAllocaFails
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-nonentry-unsupported.ll"
    ERROR
  )

  goallc_add_failure_filecheck_test(
    GoALLCStatepoints.DynamicPointerAllocaFails
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-dynamic-unsupported.ll"
    ERROR
  )

  goallc_add_failure_filecheck_test(
    GoALLCStatepoints.RealignedPointerAllocaFails
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-realigned-unsupported.ll"
    ERROR
  )

  goallc_add_failure_filecheck_test(
    GoALLCStatepoints.PointerVectorAllocaFails
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-vector-unsupported.ll"
    ERROR
  )

  goallc_add_ir_filecheck_test(
    GoALLCStatepoints.PointerAllocaLifetimeSupported
    "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-lifetime-unsupported.ll"
    IR
  )
  add_test(
    NAME GoALLCStatepoints.PointerAllocaLifetimeGoObj
    COMMAND "${GOALLC_LLC_EXECUTABLE}"
      "-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>"
      -verify-machineinstrs -filetype=obj
      -o "${CMAKE_CURRENT_BINARY_DIR}/alloca-pointer-lifetime.goobj"
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-lifetime-unsupported.ll"
  )
  if(GOALLC_OBJVIEW_EXECUTABLE)
    goallc_add_filecheck_test(
      GoALLCStatepoints.PointerAllocaLifetimeObjView
      "${CMAKE_CURRENT_SOURCE_DIR}/testdata/alloca-pointer-lifetime-unsupported.ll"
      OBJVIEW
      "${GOALLC_OBJVIEW_EXECUTABLE}" -json
      "${CMAKE_CURRENT_BINARY_DIR}/alloca-pointer-lifetime.goobj"
    )
    set_tests_properties(
      GoALLCStatepoints.PointerAllocaLifetimeObjView PROPERTIES
      DEPENDS GoALLCStatepoints.PointerAllocaLifetimeGoObj
    )
  endif()

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

  set(GOALLC_AGGREGATE_CFG_PIPELINE
    "'${GOALLC_LLC_EXECUTABLE}'"
    " '-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>'"
    " -goallc-pass-plugin-emit-ir -filetype=null -o -"
    " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-cfg.ll'"
    " 2>&1"
    " | '${GOALLC_OPT_EXECUTABLE}'"
    " '-load-pass-plugin=$<TARGET_FILE:GoALLCStatepoints>'"
    " -S -passes=verify -o - -"
    " | '${GOALLC_FILECHECK_EXECUTABLE}' --check-prefix=IR"
    " '${CMAKE_CURRENT_SOURCE_DIR}/testdata/aggregate-cfg.ll'")
  string(JOIN "" GOALLC_AGGREGATE_CFG_PIPELINE
    ${GOALLC_AGGREGATE_CFG_PIPELINE})
  add_test(
    NAME GoALLCStatepoints.AggregateCFGRewrite
    COMMAND "${GOALLC_BASH_EXECUTABLE}" -o pipefail -c
      "${GOALLC_AGGREGATE_CFG_PIPELINE}"
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
