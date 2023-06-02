--| name: Name
--| description: Description
--| version: 0.1.0

local http = require"http"

function SearchMangas(query)
    return {
        {
            title = query
        }
    }
end

function MangaChapters(manga)
    return {
        {
            title = "Chapter 1"
        }
    }
end

function ChapterPages(chapter)
    local headers = {
        ["Referer"] = "https://example.com",
        ["User-Agent"] = "libmangal",
    }

    return {
        {
            url = "https://example.com/image.jpg",
            headers = headers,
        }
    }
end