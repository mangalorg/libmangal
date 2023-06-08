--| name: Mangapill
--| description: Provider for mangapill.com
--| version: v0.1.0

local mangal = require('libmangal')
local http = mangal.http
local base_url = "https://mangapill.com"
local user_agent = "libmangal"

local function query_to_search_url(query)
    query = mangal.strings.replace_all(query, " ", "+")
    query = query:lower()
    query = mangal.strings.trim_space(query)

    local params = mangal.urls.values()
    params:set("q", query)
    params:set("type", "")
    params:set("status", "")

    return base_url .. "/search?" .. params:encode()
end

function SearchMangas(query)
    local url = query_to_search_url(query)

    local req = http.request(http.MethodGet, url)
    req:header("User-Agent", user_agent)
    local res = req:send()

    if res:status() ~= http.StatusOK then
        error(res:status())
    end

    local html = mangal.html.parse(res:body())

    local mangas = {}

    local selector =
    'body > div.container.py-3 > div.my-3.grid.justify-end.gap-3.grid-cols-2.md\\:grid-cols-3.lg\\:grid-cols-5 > div'
    html:find(selector):each(function(selection)
        local title = selection:find('div a div.leading-tight'):text()
        local href = selection:find('div a:first-child'):attr_or('href', '')
        local cover = selection:find('img'):attr_or('data-src', '')
        local id = mangal.strings.split(href, '/')[3]

        local manga = {
            title = title,
            url = base_url .. href,
            cover = cover,
            id = id,
        }

        table.insert(mangas, manga)
    end)


    return mangas
end

function MangaVolumes(manga)
    local req = http.request(http.MethodGet, manga.url)
    req:header("User-Agent", user_agent)
    local res = req:send()

    if res:status() ~= http.StatusOK then
        error(res:status())
    end

    local html = mangal.html.parse(res:body())

    local chapters = {}
    local selector = "div[data-filter-list] a"
    html:find(selector):each(function(selection)
        local title = mangal.strings.trim_space(selection:text())
        local url = selection:attr_or('href', '')
        local number = mangal.strings.split(title, " ")[2]

        local chapter = {
            title = title,
            url = base_url .. url,
            number = number
        }

        table.insert(chapters, chapter)
    end)

    chapters = mangal.util.reverse(chapters)

    return {
        {
            number = 1,
            chapters = chapters,
        }
    }
end

function VolumeChapters(volume)
    return volume.chapters
end

function ChapterPages(chapter)
    local req = http.request(http.MethodGet, chapter.url)
    req:header("User-Agent", user_agent)
    local res = req:send()

    if res:status() ~= http.StatusOK then
        error(res:status())
    end

    local html = mangal.html.parse(res:body())

    local pages = {}
    local selector = "picture img"
    html:find(selector):each(function(selection)
        local url = selection:attr_or('data-src', '')

        local page = {
            url = url,
        }

        table.insert(pages, page)
    end)

    return pages
end
