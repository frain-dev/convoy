# Set version
tag=$1
etag="$tag Enterprise Edition"
: > ./VERSION && echo $tag >  VERSION
: > ./ee/VERSION && echo $etag > ./ee/VERSION

# Commit version number & push
git add VERSION ./ee/VERSION
git commit -m "Bump version to $tag"
git push origin

# Tag & Push.
git tag $tag
git push origin $tag
