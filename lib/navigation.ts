export type NavigationItem = {
  href: string;
  label: string;
  mobileLabel: string;
};

export const navigationItems: NavigationItem[] = [
  { href: "/overview", label: "Overview", mobileLabel: "Overview" },
  { href: "/progress", label: "Progress", mobileLabel: "Progress" },
  { href: "/settings", label: "Settings", mobileLabel: "Settings" },
  { href: "/profile", label: "Profile", mobileLabel: "Profile" },
];
