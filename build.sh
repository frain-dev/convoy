# Build UI.

# Enter UI directory
cd ./web/ui/dashboard

# Install dependencies
npm install

# Run production build
npm run build

# Copy build artifacts
cd ../../../
mv web/ui/dashboard/dist/* server/ui/build

# Build Binary
go build -o convoy ./cmd/*.go