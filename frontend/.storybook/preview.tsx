import type { Decorator, Preview } from "@storybook/react-vite";
// index.css loads Tailwind + Forge tokens so stories render on the real page
// background via var(--gf-bg-page), with every Forge var / utility resolved.
// We deliberately do NOT configure a `backgrounds` palette: the Storybook
// addon needs literal color values, which would mean hardcoding hex (banned
// by ds-purity) — and a token switcher belongs in a decorator, not here.
import "../src/app/index.css";

// Theme decorator — sets both `data-theme` attribute and `.dark` class on
// <html> so Forge's .dark cascade (surfaces/text/borders/shadows) fires
// alongside Margince's brand-override block in ledger-green.css.
const withTheme: Decorator = (Story, context) => {
  const theme = context.globals.theme as string;
  const html = document.documentElement;
  if (theme === "dark") {
    html.dataset.theme = "dark";
    html.classList.add("dark");
  } else {
    delete html.dataset.theme;
    html.classList.remove("dark");
  }
  return <Story />;
};

// Surface decorator — frames EVERY story on the real `bg-gf-page` surface with
// consistent breathing room, so the catalog reads as deliberately composed
// rather than components dumped in the canvas corner. Layout is centralized
// here, not sprinkled per-story. Opt out via `parameters.surface`:
//   "padded"     (default) — page surface + p-gf-lg gutter, content top-left
//   "centered"   — page surface, content centered (best for single atoms)
//   "fullscreen" — page surface, no gutter (full-bleed shells / boards)
const withSurface: Decorator = (Story, context) => {
  const surface = (context.parameters.surface as string) ?? "padded";
  const cls =
    surface === "fullscreen"
      ? "min-h-screen bg-gf-page"
      : surface === "centered"
        ? "min-h-screen bg-gf-page flex items-center justify-center p-gf-lg"
        : "min-h-screen bg-gf-page p-gf-lg";
  return (
    <div className={cls}>
      <Story />
    </div>
  );
};

const preview: Preview = {
  parameters: {
    controls: { matchers: { color: /(background|color)$/i, date: /Date$/i } },
  },

  globalTypes: {
    theme: {
      description: "Ledger-Green color theme",
      defaultValue: "light",
      toolbar: {
        title: "Theme",
        icon: "circlehollow",
        items: [
          { value: "light", icon: "sun", title: "Light" },
          { value: "dark", icon: "moon", title: "Dark" },
        ],
        dynamicTitle: true,
      },
    },
  },

  // Order matters: theme sets the cascade on <html>, surface frames the story.
  decorators: [withSurface, withTheme],
};

export default preview;
