; Shared pointer derivations must be traversed by node, not by path.
; Thirty diamonds have only 90 pointer derivations but 2^30 paths.
; The ordinary graph exercises marker reachability; the marked graph
; exercises the all-inputs query after marker reachability succeeds.
;
; CHECK-LABEL: define goabiinternal i64 @ordinary_graph(
; CHECK: llvm.experimental.gc.statepoint{{.*}}"gc-live"(
; CHECK: llvm.experimental.gc.relocate
; CHECK: ret i64
;
; CHECK-LABEL: define goabiinternal i64 @marked_graph(
; CHECK-NOT: "gc-live"
; CHECK: llvm.experimental.gc.statepoint
; CHECK-NOT: "gc-live"
; CHECK-NOT: llvm.experimental.gc.relocate
; CHECK: ret i64
;
; CHECK-LABEL: define goabiinternal i64 @marked_cycle(
; CHECK-NOT: "gc-live"
; CHECK: llvm.experimental.gc.statepoint
; CHECK-NOT: "gc-live"
; CHECK-NOT: llvm.experimental.gc.relocate
; CHECK: ret i64
;
; CHECK-LABEL: define goabiinternal i64 @unmarked_cycle(
; CHECK: llvm.experimental.gc.statepoint{{.*}}"gc-live"(
; CHECK: llvm.experimental.gc.relocate
; CHECK: ret i64
;
; CHECK-LABEL: define goabiinternal i64 @mixed_cycle(
; CHECK: llvm.experimental.gc.statepoint{{.*}}"gc-live"(
; CHECK: llvm.experimental.gc.relocate
; CHECK: ret i64
;
target triple = "aarch64-apple-darwin-goobj"
declare goabiinternal void @callee()

define goabiinternal i64 @ordinary_graph(ptr %ordinary, i1 %choose) gc "goallc" {
entry:
  %left1 = getelementptr i8, ptr %ordinary, i64 1
  %right1 = getelementptr i8, ptr %ordinary, i64 2
  %p1 = select i1 %choose, ptr %left1, ptr %right1
  %left2 = getelementptr i8, ptr %p1, i64 1
  %right2 = getelementptr i8, ptr %p1, i64 2
  %p2 = select i1 %choose, ptr %left2, ptr %right2
  %left3 = getelementptr i8, ptr %p2, i64 1
  %right3 = getelementptr i8, ptr %p2, i64 2
  %p3 = select i1 %choose, ptr %left3, ptr %right3
  %left4 = getelementptr i8, ptr %p3, i64 1
  %right4 = getelementptr i8, ptr %p3, i64 2
  %p4 = select i1 %choose, ptr %left4, ptr %right4
  %left5 = getelementptr i8, ptr %p4, i64 1
  %right5 = getelementptr i8, ptr %p4, i64 2
  %p5 = select i1 %choose, ptr %left5, ptr %right5
  %left6 = getelementptr i8, ptr %p5, i64 1
  %right6 = getelementptr i8, ptr %p5, i64 2
  %p6 = select i1 %choose, ptr %left6, ptr %right6
  %left7 = getelementptr i8, ptr %p6, i64 1
  %right7 = getelementptr i8, ptr %p6, i64 2
  %p7 = select i1 %choose, ptr %left7, ptr %right7
  %left8 = getelementptr i8, ptr %p7, i64 1
  %right8 = getelementptr i8, ptr %p7, i64 2
  %p8 = select i1 %choose, ptr %left8, ptr %right8
  %left9 = getelementptr i8, ptr %p8, i64 1
  %right9 = getelementptr i8, ptr %p8, i64 2
  %p9 = select i1 %choose, ptr %left9, ptr %right9
  %left10 = getelementptr i8, ptr %p9, i64 1
  %right10 = getelementptr i8, ptr %p9, i64 2
  %p10 = select i1 %choose, ptr %left10, ptr %right10
  %left11 = getelementptr i8, ptr %p10, i64 1
  %right11 = getelementptr i8, ptr %p10, i64 2
  %p11 = select i1 %choose, ptr %left11, ptr %right11
  %left12 = getelementptr i8, ptr %p11, i64 1
  %right12 = getelementptr i8, ptr %p11, i64 2
  %p12 = select i1 %choose, ptr %left12, ptr %right12
  %left13 = getelementptr i8, ptr %p12, i64 1
  %right13 = getelementptr i8, ptr %p12, i64 2
  %p13 = select i1 %choose, ptr %left13, ptr %right13
  %left14 = getelementptr i8, ptr %p13, i64 1
  %right14 = getelementptr i8, ptr %p13, i64 2
  %p14 = select i1 %choose, ptr %left14, ptr %right14
  %left15 = getelementptr i8, ptr %p14, i64 1
  %right15 = getelementptr i8, ptr %p14, i64 2
  %p15 = select i1 %choose, ptr %left15, ptr %right15
  %left16 = getelementptr i8, ptr %p15, i64 1
  %right16 = getelementptr i8, ptr %p15, i64 2
  %p16 = select i1 %choose, ptr %left16, ptr %right16
  %left17 = getelementptr i8, ptr %p16, i64 1
  %right17 = getelementptr i8, ptr %p16, i64 2
  %p17 = select i1 %choose, ptr %left17, ptr %right17
  %left18 = getelementptr i8, ptr %p17, i64 1
  %right18 = getelementptr i8, ptr %p17, i64 2
  %p18 = select i1 %choose, ptr %left18, ptr %right18
  %left19 = getelementptr i8, ptr %p18, i64 1
  %right19 = getelementptr i8, ptr %p18, i64 2
  %p19 = select i1 %choose, ptr %left19, ptr %right19
  %left20 = getelementptr i8, ptr %p19, i64 1
  %right20 = getelementptr i8, ptr %p19, i64 2
  %p20 = select i1 %choose, ptr %left20, ptr %right20
  %left21 = getelementptr i8, ptr %p20, i64 1
  %right21 = getelementptr i8, ptr %p20, i64 2
  %p21 = select i1 %choose, ptr %left21, ptr %right21
  %left22 = getelementptr i8, ptr %p21, i64 1
  %right22 = getelementptr i8, ptr %p21, i64 2
  %p22 = select i1 %choose, ptr %left22, ptr %right22
  %left23 = getelementptr i8, ptr %p22, i64 1
  %right23 = getelementptr i8, ptr %p22, i64 2
  %p23 = select i1 %choose, ptr %left23, ptr %right23
  %left24 = getelementptr i8, ptr %p23, i64 1
  %right24 = getelementptr i8, ptr %p23, i64 2
  %p24 = select i1 %choose, ptr %left24, ptr %right24
  %left25 = getelementptr i8, ptr %p24, i64 1
  %right25 = getelementptr i8, ptr %p24, i64 2
  %p25 = select i1 %choose, ptr %left25, ptr %right25
  %left26 = getelementptr i8, ptr %p25, i64 1
  %right26 = getelementptr i8, ptr %p25, i64 2
  %p26 = select i1 %choose, ptr %left26, ptr %right26
  %left27 = getelementptr i8, ptr %p26, i64 1
  %right27 = getelementptr i8, ptr %p26, i64 2
  %p27 = select i1 %choose, ptr %left27, ptr %right27
  %left28 = getelementptr i8, ptr %p27, i64 1
  %right28 = getelementptr i8, ptr %p27, i64 2
  %p28 = select i1 %choose, ptr %left28, ptr %right28
  %left29 = getelementptr i8, ptr %p28, i64 1
  %right29 = getelementptr i8, ptr %p28, i64 2
  %p29 = select i1 %choose, ptr %left29, ptr %right29
  %left30 = getelementptr i8, ptr %p29, i64 1
  %right30 = getelementptr i8, ptr %p29, i64 2
  %p30 = select i1 %choose, ptr %left30, ptr %right30
  call goabiinternal void @callee()
  %value = load i64, ptr %p30, align 1
  ret i64 %value
}

define goabiinternal i64 @marked_graph(i64 %address, i1 %choose) gc "goallc" {
entry:
  %base = inttoptr i64 %address to ptr, !goallc.notinheap !0
  %left1 = getelementptr i8, ptr %base, i64 1
  %right1 = getelementptr i8, ptr %base, i64 2
  %p1 = select i1 %choose, ptr %left1, ptr %right1
  %left2 = getelementptr i8, ptr %p1, i64 1
  %right2 = getelementptr i8, ptr %p1, i64 2
  %p2 = select i1 %choose, ptr %left2, ptr %right2
  %left3 = getelementptr i8, ptr %p2, i64 1
  %right3 = getelementptr i8, ptr %p2, i64 2
  %p3 = select i1 %choose, ptr %left3, ptr %right3
  %left4 = getelementptr i8, ptr %p3, i64 1
  %right4 = getelementptr i8, ptr %p3, i64 2
  %p4 = select i1 %choose, ptr %left4, ptr %right4
  %left5 = getelementptr i8, ptr %p4, i64 1
  %right5 = getelementptr i8, ptr %p4, i64 2
  %p5 = select i1 %choose, ptr %left5, ptr %right5
  %left6 = getelementptr i8, ptr %p5, i64 1
  %right6 = getelementptr i8, ptr %p5, i64 2
  %p6 = select i1 %choose, ptr %left6, ptr %right6
  %left7 = getelementptr i8, ptr %p6, i64 1
  %right7 = getelementptr i8, ptr %p6, i64 2
  %p7 = select i1 %choose, ptr %left7, ptr %right7
  %left8 = getelementptr i8, ptr %p7, i64 1
  %right8 = getelementptr i8, ptr %p7, i64 2
  %p8 = select i1 %choose, ptr %left8, ptr %right8
  %left9 = getelementptr i8, ptr %p8, i64 1
  %right9 = getelementptr i8, ptr %p8, i64 2
  %p9 = select i1 %choose, ptr %left9, ptr %right9
  %left10 = getelementptr i8, ptr %p9, i64 1
  %right10 = getelementptr i8, ptr %p9, i64 2
  %p10 = select i1 %choose, ptr %left10, ptr %right10
  %left11 = getelementptr i8, ptr %p10, i64 1
  %right11 = getelementptr i8, ptr %p10, i64 2
  %p11 = select i1 %choose, ptr %left11, ptr %right11
  %left12 = getelementptr i8, ptr %p11, i64 1
  %right12 = getelementptr i8, ptr %p11, i64 2
  %p12 = select i1 %choose, ptr %left12, ptr %right12
  %left13 = getelementptr i8, ptr %p12, i64 1
  %right13 = getelementptr i8, ptr %p12, i64 2
  %p13 = select i1 %choose, ptr %left13, ptr %right13
  %left14 = getelementptr i8, ptr %p13, i64 1
  %right14 = getelementptr i8, ptr %p13, i64 2
  %p14 = select i1 %choose, ptr %left14, ptr %right14
  %left15 = getelementptr i8, ptr %p14, i64 1
  %right15 = getelementptr i8, ptr %p14, i64 2
  %p15 = select i1 %choose, ptr %left15, ptr %right15
  %left16 = getelementptr i8, ptr %p15, i64 1
  %right16 = getelementptr i8, ptr %p15, i64 2
  %p16 = select i1 %choose, ptr %left16, ptr %right16
  %left17 = getelementptr i8, ptr %p16, i64 1
  %right17 = getelementptr i8, ptr %p16, i64 2
  %p17 = select i1 %choose, ptr %left17, ptr %right17
  %left18 = getelementptr i8, ptr %p17, i64 1
  %right18 = getelementptr i8, ptr %p17, i64 2
  %p18 = select i1 %choose, ptr %left18, ptr %right18
  %left19 = getelementptr i8, ptr %p18, i64 1
  %right19 = getelementptr i8, ptr %p18, i64 2
  %p19 = select i1 %choose, ptr %left19, ptr %right19
  %left20 = getelementptr i8, ptr %p19, i64 1
  %right20 = getelementptr i8, ptr %p19, i64 2
  %p20 = select i1 %choose, ptr %left20, ptr %right20
  %left21 = getelementptr i8, ptr %p20, i64 1
  %right21 = getelementptr i8, ptr %p20, i64 2
  %p21 = select i1 %choose, ptr %left21, ptr %right21
  %left22 = getelementptr i8, ptr %p21, i64 1
  %right22 = getelementptr i8, ptr %p21, i64 2
  %p22 = select i1 %choose, ptr %left22, ptr %right22
  %left23 = getelementptr i8, ptr %p22, i64 1
  %right23 = getelementptr i8, ptr %p22, i64 2
  %p23 = select i1 %choose, ptr %left23, ptr %right23
  %left24 = getelementptr i8, ptr %p23, i64 1
  %right24 = getelementptr i8, ptr %p23, i64 2
  %p24 = select i1 %choose, ptr %left24, ptr %right24
  %left25 = getelementptr i8, ptr %p24, i64 1
  %right25 = getelementptr i8, ptr %p24, i64 2
  %p25 = select i1 %choose, ptr %left25, ptr %right25
  %left26 = getelementptr i8, ptr %p25, i64 1
  %right26 = getelementptr i8, ptr %p25, i64 2
  %p26 = select i1 %choose, ptr %left26, ptr %right26
  %left27 = getelementptr i8, ptr %p26, i64 1
  %right27 = getelementptr i8, ptr %p26, i64 2
  %p27 = select i1 %choose, ptr %left27, ptr %right27
  %left28 = getelementptr i8, ptr %p27, i64 1
  %right28 = getelementptr i8, ptr %p27, i64 2
  %p28 = select i1 %choose, ptr %left28, ptr %right28
  %left29 = getelementptr i8, ptr %p28, i64 1
  %right29 = getelementptr i8, ptr %p28, i64 2
  %p29 = select i1 %choose, ptr %left29, ptr %right29
  %left30 = getelementptr i8, ptr %p29, i64 1
  %right30 = getelementptr i8, ptr %p29, i64 2
  %p30 = select i1 %choose, ptr %left30, ptr %right30
  call goabiinternal void @callee()
  %value = load i64, ptr %p30, align 1
  ret i64 %value
}

define goabiinternal i64 @marked_cycle(i64 %address, ptr %ordinary, i1 %again, i1 %choose) gc "goallc" {
entry:
  %base = inttoptr i64 %address to ptr, !goallc.notinheap !0
  br label %loop
loop:
  ; Put the backedge first: encountering an active cycle must not cache
  ; a negative answer before the marker-bearing incoming edge is explored.
  %cursor = phi ptr [ %next, %loop ], [ %base, %entry ]
  call goabiinternal void @callee()
  %value = load i64, ptr %cursor, align 1
  %derived = getelementptr i8, ptr %cursor, i64 1
  %next = getelementptr i8, ptr %derived, i64 1
  br i1 %again, label %loop, label %exit
exit:
  ret i64 %value
}

define goabiinternal i64 @unmarked_cycle(i64 %address, ptr %ordinary, i1 %again, i1 %choose) gc "goallc" {
entry:
  %base = inttoptr i64 %address to ptr
  br label %loop
loop:
  ; Put the backedge first: encountering an active cycle must not cache
  ; a negative answer before the marker-bearing incoming edge is explored.
  %cursor = phi ptr [ %next, %loop ], [ %base, %entry ]
  call goabiinternal void @callee()
  %value = load i64, ptr %cursor, align 1
  %derived = getelementptr i8, ptr %cursor, i64 1
  %next = getelementptr i8, ptr %derived, i64 1
  br i1 %again, label %loop, label %exit
exit:
  ret i64 %value
}

define goabiinternal i64 @mixed_cycle(i64 %address, ptr %ordinary, i1 %again, i1 %choose) gc "goallc" {
entry:
  %base = inttoptr i64 %address to ptr, !goallc.notinheap !0
  br label %loop
loop:
  ; Put the backedge first: encountering an active cycle must not cache
  ; a negative answer before the marker-bearing incoming edge is explored.
  %cursor = phi ptr [ %next, %loop ], [ %base, %entry ]
  call goabiinternal void @callee()
  %value = load i64, ptr %cursor, align 1
  %derived = getelementptr i8, ptr %cursor, i64 1
  %next = select i1 %choose, ptr %derived, ptr %ordinary
  br i1 %again, label %loop, label %exit
exit:
  ret i64 %value
}

!0 = !{}
