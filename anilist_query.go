package libmangal

// anilistQueryCommon common manga query used for getting manga by id or searching it by name
const anilistQueryCommon = `
id
idMal
title {
	romaji
	english
	native
}
description(asHtml: false)
averageScore
tags {
	name
	description
	rank
}
genres
coverImage {
	extraLarge
	large
	medium
	color
}
bannerImage
characters (page: 1, perPage: 10, role: MAIN) {
	nodes {
		id
		name {
			full
			native
		}
	}
}
startDate {
	year
	month	
	day
}
endDate {
	year
	month	
	day
}
staff {
	edges {
	  role
	  node {
		name {
		  full
		}
	  }
	}
}
status
synonyms
siteUrl
chapters
countryOfOrigin
externalLinks {
	url
}
`

const anilistQuerySearchByName = `
query ($query: String) {
	Page (page: 1, perPage: 30) {
		media (search: $query, type: MANGA) {
			` + anilistQueryCommon + `
		}
	}
}`

const anilistQuerySearchByID = `
query ($id: Int) {
	Media (id: $id, type: MANGA) {
		` + anilistQueryCommon + `
	}
}`
