import "./globals.css";

export const metadata = {
  title: "URL Shortener + Analytics",
  description: "Shorten links and track clicks",
};

// RootLayout wraps every page with the global stylesheet and the top nav bar.
export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>
        <header className="header">
          <a href="/" className="brand">🔗 Shortly</a>
        </header>
        <main className="container">{children}</main>
      </body>
    </html>
  );
}
