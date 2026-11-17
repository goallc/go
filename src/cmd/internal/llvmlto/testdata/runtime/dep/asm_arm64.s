#include "textflag.h"

TEXT ·Assembly(SB), $0-0
	CALL ·callback(SB)
	RET
