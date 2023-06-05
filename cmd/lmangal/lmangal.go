package main

import (
	"context"
	"fmt"
	"github.com/mangalorg/libmangal"
	"github.com/mangalorg/libmangal/vm"
	"github.com/mangalorg/libmangal/vm/lib"
	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/bbolt"
	"github.com/spf13/cobra"
	"os"
)

var client = newClient()

func newClient() *libmangal.Client {
	options := libmangal.DefaultClientOptions()
	options.Log = func(s string) {
		fmt.Println(s)
	}

	newPersistentStore := func(name string) gokv.Store {
		options := bbolt.DefaultOptions
		options.BucketName = name
		options.Path = name + ".db"

		store, err := bbolt.NewStore(options)
		if err != nil {
			panic("cannot create persistent store")
		}

		return store
	}

	options.Anilist.QueryToIdsStore = newPersistentStore("query-to-ids")
	options.Anilist.TitleToIdStore = newPersistentStore("title-to-id")
	options.Anilist.IdToMangaStore = newPersistentStore("id-to-manga")

	return libmangal.NewClient(options)
}

var rootCmd = &cobra.Command{
	Use:   "lmangal",
	Short: "lmangal is a command line tool for libmangal",
	Args:  cobra.NoArgs,
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringP("query", "q", "", "query string to search mangas")
	runCmd.Flags().IntP("manga", "m", 0, "manga index to download")
	runCmd.Flags().IntP("chapter", "c", 0, "chapter index to download")
	runCmd.Flags().BoolP("download", "d", false, "download chapter")
	runCmd.Flags().BoolP("read", "r", false, "read chapter")
	runCmd.Flags().StringP("format", "f", "images", "format to download chapter")
	runCmd.Flags().BoolP("list", "l", false, "list found items")

	runCmd.MarkFlagsMutuallyExclusive("download", "read")
}

var runCmd = &cobra.Command{
	Use:   "run <provider path>",
	Short: "Run a provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		handle, err := client.ProviderHandleFromPath(args[0])
		if err != nil {
			return err
		}

		provider, err := handle.LoadProvider(nil)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		err = provider.Load(context.Background())
		if err != nil {
			return err
		}

		if query, _ := cmd.Flags().GetString("query"); query != "" {
			mangas, err := provider.SearchMangas(context.Background(), query)
			if err != nil {
				return err
			}

			list, _ := cmd.Flags().GetBool("list")

			if list {
				for _, manga := range mangas {
					fmt.Println(manga)
				}
			}

			if mangaIndex, _ := cmd.Flags().GetInt("manga"); mangaIndex != 0 {
				fmt.Println()
				if mangaIndex > 0 {
					mangaIndex--
				}

				if mangaIndex >= len(mangas) {
					return fmt.Errorf("manga index out of range")
				}

				manga := mangas[mangaIndex]
				chapters, err := provider.MangaChapters(context.Background(), manga)
				if err != nil {
					return err
				}

				if list {
					for _, chapter := range chapters {
						fmt.Println(chapter)
					}
				}

				if chapterIndex, _ := cmd.Flags().GetInt("chapter"); chapterIndex != 0 {
					fmt.Println()
					if chapterIndex > 0 {
						chapterIndex--
					}

					if chapterIndex >= len(chapters) {
						return fmt.Errorf("chapter index out of range")
					}

					chapter := chapters[chapterIndex]

					if download, _ := cmd.Flags().GetBool("download"); download {
						rawFormat, _ := cmd.Flags().GetString("format")
						format, err := libmangal.FormatString(rawFormat)
						if err != nil {
							return err
						}

						downloadOptions := libmangal.DefaultDownloadOptions()
						downloadOptions.Format = format
						downloadOptions.WriteSeriesJson = true
						downloadOptions.WriteComicInfoXml = true

						_, err = provider.DownloadChapter(context.Background(), chapter, ".", downloadOptions)
						if err != nil {
							return err
						}
					} else if read, _ := cmd.Flags().GetBool("read"); read {
						rawFormat, _ := cmd.Flags().GetString("format")
						format, err := libmangal.FormatString(rawFormat)
						if err != nil {
							return err
						}

						readOptions := libmangal.DefaultReadOptions()
						readOptions.Format = format
						readOptions.MangasLibraryPath = "."
						err = provider.ReadChapter(context.Background(), chapter, readOptions)

						if err != nil {
							return err
						}
					} else {
						pages, err := provider.ChapterPages(context.Background(), chapter)
						if err != nil {
							return err
						}

						if list {
							for _, page := range pages {
								fmt.Println(page)
							}
						}
					}
				}
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(probeCmd)
}

var probeCmd = &cobra.Command{
	Use:   "probe <provider path>",
	Short: "Probe a provider",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		handle, err := client.ProviderHandleFromPath(args[0])
		if err != nil {
			return err
		}

		provider, err := handle.LoadProvider(nil)
		if err != nil {
			return err
		}

		info := provider.Info()
		fmt.Printf(`Name: %s
Description: %s
Version: %s
`,
			info.Name,
			info.Description,
			info.Version,
		)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(docCmd)
}

var docCmd = &cobra.Command{
	Use:   "doc",
	Short: "Generate documentation",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		doc := lib.Lib(vm.NewState(vm.DefaultOptions()), lib.DefaultOptions()).LuaDoc()

		fmt.Println(doc)
	},
}

func init() {
	rootCmd.AddCommand(anilistCmd)
}

var anilistCmd = &cobra.Command{
	Use:   "anilist",
	Short: "Anilist related commands",
}

func init() {
	anilistCmd.AddCommand(anilistSearchCmd)
	anilistSearchCmd.Flags().BoolP("closest", "c", false, "Get the closest match")
}

var anilistSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search for an anime",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flag("closest").Changed {
			manga, err := client.AnilistFindClosestManga(context.Background(), args[0])
			if err != nil {
				return err
			}

			fmt.Println(manga)
			fmt.Println(manga.Description)
			return nil
		}

		mangas, err := client.AnilistSearchManga(context.Background(), args[0])
		if err != nil {
			return err
		}

		for _, manga := range mangas {
			fmt.Println(manga)
		}

		return nil
	},
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
