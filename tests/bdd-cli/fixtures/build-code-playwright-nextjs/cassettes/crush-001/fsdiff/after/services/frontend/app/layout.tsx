import type { ReactNode } from 'react';

export const metadata = {
  title: 'Frontend',
  description: 'Frontend home page',
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
