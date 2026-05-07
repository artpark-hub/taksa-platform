import type { Metadata } from "next";
import "./globals.css";
import "../components/Login.css";
import "../components/Register.css";
import "../components/ForgotPassword.css";
import "../components/LegalDocuments.css";

export const metadata: Metadata = {
  title: "Taksa",
  description: "Open Source Factory OS",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}