// The top row's height: tall enough to centre the native window buttons, which the window
// puts where it puts them (state.controls); with none, as in full screen, a tab's height.
// The tabs on top sit at its foot, 30px, and the space above them moves the window.
export const TAB = 30;

export function topRow(state) {
  return state.controls.h > 0 ? state.controls.y * 2 + state.controls.h : TAB;
}
