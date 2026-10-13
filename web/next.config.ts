const nextConfig = {
  output: "export",
  distDir: "../embed/out",
  images: { unoptimized: true },
  async rewrites() {
    return process.env.NODE_ENV === "development"
      ? [{ source: "/api/:path*", destination: "http://localhost:5001/api/:path*" }]
      : [];
  },
};
export default nextConfig;