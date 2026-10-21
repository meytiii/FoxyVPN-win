// High-quality SVG flag representations for VPN exit countries

// Embedded SVG data URIs for instant 0ms offline rendering of core countries
const LOCAL_FLAGS: Record<string, string> = {
  // United States
  us: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23bd3d44" d="M0 0h640v480H0z"/><path stroke="%23fff" stroke-width="37" d="M0 55.4h640M0 129.2h640M0 203h640M0 277h640M0 350.8h640M0 424.6h640"/><path fill="%23192f5d" d="M0 0h256v258.5H0z"/><circle cx="128" cy="129" r="80" fill="%23fff" opacity="0.15"/></svg>`,
  // Germany
  de: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23ffce00" d="M0 320h640v160H0z"/><path fill="%23000" d="M0 0h640v160H0z"/><path fill="%23dd0000" d="M0 160h640v160H0z"/></svg>`,
  // United Kingdom
  gb: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23012169" d="M0 0h640v480H0z"/><path stroke="%23fff" stroke-width="60" d="M0 0l640 480M640 0L0 480"/><path stroke="%23c8102e" stroke-width="40" d="M0 0l640 480M640 0L0 480"/><path stroke="%23fff" stroke-width="100" d="M320 0v480M0 240h640"/><path stroke="%23c8102e" stroke-width="60" d="M320 0v480M0 240h640"/></svg>`,
  // Netherlands
  nl: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%2321468b" d="M0 320h640v160H0z"/><path fill="%23fff" d="M0 160h640v160H0z"/><path fill="%23ae1c28" d="M0 0h640v160H0z"/></svg>`,
  // France
  fr: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23002654" d="M0 0h213.3v480H0z"/><path fill="%23ce1126" d="M426.7 0H640v480H426.7z"/><path fill="%23fff" d="M213.3 0h213.4v480H213.3z"/></svg>`,
  // Canada
  ca: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23d80621" d="M0 0h160v480H0zM480 0h160v480H480z"/><path fill="%23fff" d="M160 0h320v480H160z"/><path fill="%23d80621" d="M320 90l25 60 55-20-15 65 60 15-40 40 5 60-60-20-30 60-30-60-60 20 5-60-40-40 60-15-15-65 55 20z"/></svg>`,
  // Japan
  jp: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23fff" d="M0 0h640v480H0z"/><circle cx="320" cy="240" r="144" fill="%23bc002d"/></svg>`,
  // Australia
  au: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%2300008b" d="M0 0h640v480H0z"/><path stroke="%23fff" stroke-width="30" d="M0 0l320 240M320 0L0 240"/><path stroke="%23cc0000" stroke-width="20" d="M0 0l320 240M320 0L0 240"/><path stroke="%23fff" stroke-width="50" d="M160 0v240M0 120h320"/><path stroke="%23cc0000" stroke-width="30" d="M160 0v240M0 120h320"/><circle cx="480" cy="240" r="25" fill="%23fff"/><circle cx="540" cy="140" r="15" fill="%23fff"/><circle cx="420" cy="360" r="15" fill="%23fff"/><circle cx="540" cy="340" r="15" fill="%23fff"/></svg>`,
  // Switzerland
  ch: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23d52b1e" d="M0 0h640v480H0z"/><path fill="%23fff" d="M260 100h120v280H260z"/><path fill="%23fff" d="M180 180h280v120H180z"/></svg>`,
  // Sweden
  se: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23006aa7" d="M0 0h640v480H0z"/><path fill="%23fecc00" d="M180 0h80v480h-80zM0 200h640v80H0z"/></svg>`,
  // Italy
  it: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23009246" d="M0 0h213.3v480H0z"/><path fill="%23ce2b37" d="M426.7 0H640v480H426.7z"/><path fill="%23fff" d="M213.3 0h213.4v480H213.3z"/></svg>`,
  // Spain
  es: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23c60b1e" d="M0 0h640v120H0zM0 360h640v120H0z"/><path fill="%23ffc400" d="M0 120h640v240H0z"/></svg>`,
  // Singapore
  sg: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23ed2939" d="M0 0h640v240H0z"/><path fill="%23fff" d="M0 240h640v240H0z"/><circle cx="120" cy="120" r="70" fill="%23fff"/><circle cx="140" cy="120" r="70" fill="%23ed2939"/></svg>`,
  // Austria
  at: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23ed2939" d="M0 0h640v160H0zM0 320h640v160H0z"/><path fill="%23fff" d="M0 160h640v160H0z"/></svg>`,
  // Belgium
  be: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23000" d="M0 0h213.3v480H0z"/><path fill="%23ed2939" d="M426.7 0H640v480H426.7z"/><path fill="%23fae042" d="M213.3 0h213.4v480H213.3z"/></svg>`,
  // Brazil
  br: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23009b3a" d="M0 0h640v480H0z"/><path fill="%23fedf00" d="M320 40L600 240 320 440 40 240z"/><circle cx="320" cy="240" r="100" fill="%23002776"/></svg>`,
  // Denmark
  dk: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23c60c30" d="M0 0h640v480H0z"/><path fill="%23fff" d="M180 0h60v480h-60zM0 210h640v60H0z"/></svg>`,
  // Finland
  fi: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23fff" d="M0 0h640v480H0z"/><path fill="%23002f6c" d="M180 0h80v480h-80zM0 200h640v80H0z"/></svg>`,
  // Ireland
  ie: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23169b62" d="M0 0h213.3v480H0z"/><path fill="%23ff883e" d="M426.7 0H640v480H426.7z"/><path fill="%23fff" d="M213.3 0h213.4v480H213.3z"/></svg>`,
  // Norway
  no: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23ba0c2f" d="M0 0h640v480H0z"/><path fill="%23fff" d="M160 0h120v480H160zM0 180h640v120H0z"/><path fill="%2300205b" d="M190 0h60v480h-60zM0 210h640v60H0z"/></svg>`,
  // Poland
  pl: `data:image/svg+xml;utf8,<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 480"><path fill="%23fff" d="M0 0h640v240H0z"/><path fill="%23dc143c" d="M0 240h640v240H0z"/></svg>`
};

/**
 * Returns flag URL (local SVG data URI or high-definition FlagCDN SVG)
 */
export function getCountryFlagUrl(countryCode: string): string {
  if (!countryCode || countryCode === 'REC' || countryCode === 'AUTO') {
    return '';
  }
  const code = countryCode.toLowerCase();
  if (LOCAL_FLAGS[code]) {
    return LOCAL_FLAGS[code];
  }
  return `https://flagcdn.com/${code}.svg`;
}
