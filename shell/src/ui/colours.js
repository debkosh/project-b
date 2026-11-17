// The frame colours to pick from, for Settings and the welcome. None is first, and is where
// a profile starts: a grey frame with ink for the accent (docs/onboarding.md). The rest are
// the frames of the Reference design's modes: Personal, Work, Kids, Open Season and Incognito,
// with Work's blue the one the Work profile template sets, so a profile made from it shows
// a preset. Go derives every other tone from whichever is picked (core/tones).
export const framePresets = [
  { name: 'None', color: '' },
  { name: 'Teal', color: '#0faaaa' },
  { name: 'Blue', color: '#5b7fd6' },
  { name: 'Amber', color: '#faa804' },
  { name: 'Red', color: '#d82b1f' },
  { name: 'Slate', color: '#384562' },
];
