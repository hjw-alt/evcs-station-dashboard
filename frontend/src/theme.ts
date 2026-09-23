/** Resolve CSS tokens to real colors: canvas charts cannot consume var(--token). */
export function readChartTheme() {
  const styles = getComputedStyle(document.documentElement)
  const color = (name: string) => styles.getPropertyValue(`--${name}`).trim()
  return {
    panel: color('panel'),
    surface: color('panel-2'),
    text: color('text'),
    muted: color('muted'),
    line: color('line'),
    lineStrong: color('line-strong'),
    primary: color('primary'),
    primaryArea: color('primary-area'),
    primarySelection: color('primary-selection'),
    green: color('green'),
    amber: color('amber'),
    red: color('red'),
    unknown: color('unknown'),
    mapArea: color('map-area'),
    mapHover: color('map-hover'),
  }
}
