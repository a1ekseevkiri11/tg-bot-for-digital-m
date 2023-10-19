package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	userInputStorage = make(map[int64]string) // Хранилище пользовательского ввода
	mutex            sync.Mutex
)

const (
	UPDATE_CONFIG_TIMEOUT   = 60
	URL_REQUEST_LESSON      = "https://www.ugrasu.ru/api/directory/lessons?fromdate="
	URL_REQUEST_LECTURER    = "https://www.ugrasu.ru/api/directory/lecturers"
	URL_REQUEST_GROUPS      = "https://www.ugrasu.ru/api/directory/groups"
	URL_REQUEST_AUDITORIUMS = "https://www.ugrasu.ru/api/directory/auditoriums"
)

func main() {
	var err error
	bot, err := tgbotapi.NewBotAPI("5798412654:AAGS0jVTr7bLLp0V2tK9ke7dv8yM1fIj9YU")
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = UPDATE_CONFIG_TIMEOUT

	for update := range bot.GetUpdatesChan(updateConfig) {

		if update.CallbackQuery != nil && update.CallbackQuery.Data != "" {
			data := update.CallbackQuery.Data
			timeTableMessage(&update, bot, userInputStorage[update.CallbackQuery.Message.Chat.ID], data)
		}

		if update.Message != nil {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, " ")
			inputUser := update.Message.Text
			inputUser = strings.TrimSpace(inputUser)
			if groupOid, found := foundGroups(inputUser); found {
				msg.Text = "Группа введена правильно!"
				saveUserInput(update.Message.Chat.ID, groupOid)
				row1 := []tgbotapi.InlineKeyboardButton{
					getKeyboardRow("Сегодня", "today_groupOid"),
					getKeyboardRow("Завтра", "tomorrow_groupOid"),
				}
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(row1)
			} else if lecturerOid, found := foundLecturer(inputUser); found {
				msg.Text = "Преподаватель найден!"
				saveUserInput(update.Message.Chat.ID, lecturerOid)
				row1 := []tgbotapi.InlineKeyboardButton{
					getKeyboardRow("Сегодня", "today_lecturerOid"),
					getKeyboardRow("Завтра", "tomorrow_lecturerOid"),
				}
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(row1)
			} else if auditoriumOid, found := foundAuditoriums(inputUser); found {
				msg.Text = "Аудитория найдена!"
				saveUserInput(update.Message.Chat.ID, auditoriumOid)
				row1 := []tgbotapi.InlineKeyboardButton{
					getKeyboardRow("Сегодня", "today_auditoriumOid"),
					getKeyboardRow("Завтра", "tomorrow_auditoriumOid"),
				}
				msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(row1)
			} else {
				msg.Text = "Ничего не нашел"
			}

			if _, err := bot.Send(msg); err != nil {
				log.Panic(err)
			}
		}
	}
}

func saveUserInput(userID int64, input string) {
	mutex.Lock()
	userInputStorage[userID] = input
	mutex.Unlock()
}

func foundAuditoriums(messageUser string) (string, bool) {
	arrayAuditoriums := requestAuditoriumsJSON()
	for i := 0; i < len(arrayAuditoriums); i++ {
		if messageUser == arrayAuditoriums[i].Name {
			return strconv.Itoa(arrayAuditoriums[i].AuditoriumOid), true
		}
	}
	return "", false
}

func foundGroups(messageUser string) (string, bool) {
	arrayGroups := requestGroupsJSON()
	for i := 0; i < len(arrayGroups); i++ {
		if messageUser == arrayGroups[i].Name {
			return strconv.Itoa(arrayGroups[i].GroupOid), true
		}
	}
	return "", false
}

func foundLecturer(messageUser string) (string, bool) {
	arrayLecturer := requestLecturerJSON()
	for i := 0; i < len(arrayLecturer); i++ {
		if messageUser == arrayLecturer[i].Fio || messageUser == arrayLecturer[i].ShortFIO {
			return strconv.Itoa(arrayLecturer[i].LecturerOid), true
		}
	}
	return "", false
}

func timeTableMessage(update *tgbotapi.Update, bot *tgbotapi.BotAPI, groupOid string, time string) {

	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Введите номер группы")

	arrayLesson := requestLessonJSON(groupOid, time)

	if len(arrayLesson) == 0 {
		msg.Text = "Расписания нет!"
		if _, err := bot.Send(msg); err != nil {
			log.Panic(err)
		}
		return
	}

	var dayWeek string
	var textMessage string

	for i := range arrayLesson {
		if dayWeek != arrayLesson[i].DayOfWeekString {
			textMessage = "Расписание на " + arrayLesson[i].DayOfWeekString + " " + arrayLesson[0].Date + "\n\n"
			dayWeek = arrayLesson[i].DayOfWeekString
		}
		textMessage += arrayLesson[i].BeginLesson + "-" + arrayLesson[i].EndLesson + "\n" + arrayLesson[i].KindOfWork + "\n" + arrayLesson[i].Discipline + "\n" + arrayLesson[i].Auditorium + "\n"
		if arrayLesson[i].LecturerRank != "!Не определена" {
			textMessage += arrayLesson[i].LecturerRank + " "
		}
		textMessage += arrayLesson[i].LecturerTitle
		if arrayLesson[i].SubGroup != "null" {
			textMessage += "\n" + string(arrayLesson[i].SubGroup)
		}
		if i != len(arrayLesson)-1 {
			textMessage += "-----------------\n"
		}
		if len(textMessage) > 2000 {
			msg.Text = textMessage
			if _, err := bot.Send(msg); err != nil {
				log.Panic(err)
			}
			textMessage = ""
		}
	}
	if len(textMessage) > 0 {
		msg.Text = textMessage
		if _, err := bot.Send(msg); err != nil {
			log.Panic(err)
		}
		textMessage = ""
	}
}

func getKeyboardRow(buttonText, buttonCode string) tgbotapi.InlineKeyboardButton {
	return tgbotapi.NewInlineKeyboardButtonData(buttonText, buttonCode)
}

func requestLessonJSON(oid string, time string) []Lesson {
	client := &http.Client{}
	request := URL_REQUEST_LESSON
	switch time {
	case "today_groupOid":
		request += todayDate() + "&todate=" + todayDate() + "&groupOid=" + oid
	case "tomorrow_groupOid":
		request += tomorrowDate() + "&todate=" + tomorrowDate() + "&groupOid=" + oid
	case "today_lecturerOid":
		request += todayDate() + "&todate=" + todayDate() + "&lectureroid=" + oid
	case "tomorrow_lecturerOid":
		request += tomorrowDate() + "&todate=" + tomorrowDate() + "&lectureroid=" + oid
	case "today_auditoriumOid":
		request += todayDate() + "&todate=" + todayDate() + "&auditoriumOid=" + oid
	case "tomorrow_auditoriumOid":
		request += tomorrowDate() + "&todate=" + tomorrowDate() + "&auditoriumOid=" + oid
	default:
		return []Lesson{}
	}
	req, err := http.NewRequest("GET", request, nil)
	if err != nil {
		return []Lesson{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return []Lesson{}
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)

	if err != nil {
		return []Lesson{}
	}

	arrayLesson := []Lesson{}

	jsonErr := json.Unmarshal(bodyText, &arrayLesson)

	if jsonErr != nil {
		return []Lesson{}
	}
	return arrayLesson
}

func requestLecturerJSON() []Lecturer {
	client := &http.Client{}
	request := URL_REQUEST_LECTURER
	req, err := http.NewRequest("GET", request, nil)
	if err != nil {
		return []Lecturer{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return []Lecturer{}
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)

	if err != nil {
		return []Lecturer{}
	}

	arrayLecturer := []Lecturer{}

	jsonErr := json.Unmarshal(bodyText, &arrayLecturer)

	if jsonErr != nil {
		return []Lecturer{}
	}
	return arrayLecturer
}

func requestGroupsJSON() []Groups {
	client := &http.Client{}
	request := URL_REQUEST_GROUPS
	req, err := http.NewRequest("GET", request, nil)
	if err != nil {
		return []Groups{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return []Groups{}
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)

	if err != nil {
		return []Groups{}
	}

	arrayGroups := []Groups{}

	jsonErr := json.Unmarshal(bodyText, &arrayGroups)

	if jsonErr != nil {
		return []Groups{}
	}
	return arrayGroups
}

func requestAuditoriumsJSON() []Auditoriums {
	client := &http.Client{}
	request := URL_REQUEST_AUDITORIUMS
	req, err := http.NewRequest("GET", request, nil)
	if err != nil {
		return []Auditoriums{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return []Auditoriums{}
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)

	if err != nil {
		return []Auditoriums{}
	}

	arrayAuditoriums := []Auditoriums{}

	jsonErr := json.Unmarshal(bodyText, &arrayAuditoriums)

	if jsonErr != nil {
		return []Auditoriums{}
	}
	return arrayAuditoriums
}

type Auditoriums struct {
	TypeOfAuditoriumOid int         `json:"TypeOfAuditoriumOid"`
	Amount              int         `json:"amount"`
	AuditoriumGUID      string      `json:"auditoriumGUID"`
	AuditoriumGid       int         `json:"auditoriumGid"`
	AuditoriumOid       int         `json:"auditoriumOid"`
	AuditoriumUID       interface{} `json:"auditoriumUID"`
	Building            string      `json:"building"`
	BuildingGid         int         `json:"buildingGid"`
	BuildingOid         int         `json:"buildingOid"`
	ComputerEquipment   int         `json:"computerEquipment"`
	Equipment           int         `json:"equipment"`
	Hideincapacity      int         `json:"hideincapacity"`
	MediaEquipment      int         `json:"mediaEquipment"`
	Name                string      `json:"name"`
	Number              string      `json:"number"`
	TableType           int         `json:"tableType"`
	TypeOfAuditorium    string      `json:"typeOfAuditorium"`
}

type Lecturer struct {
	Availability      int         `json:"availability"`
	Chair             string      `json:"chair"`
	ChairGid          int         `json:"chairGid"`
	ChairOid          int         `json:"chairOid"`
	Email             interface{} `json:"email"`
	Fio               string      `json:"fio"`
	LecturerCustomUID interface{} `json:"lecturerCustomUID"`
	LecturerGUID      string      `json:"lecturerGUID"`
	LecturerGid       int         `json:"lecturerGid"`
	LecturerOid       int         `json:"lecturerOid"`
	LecturerUID       interface{} `json:"lecturerUID"`
	LecturerRank      interface{} `json:"lecturer_rank"`
	Person            interface{} `json:"person"`
	ShortFIO          string      `json:"shortFIO"`
}

type Groups struct {
	FormOfEducationGid int         `json:"FormOfEducationGid"`
	FormOfEducationOid int         `json:"FormOfEducationOid"`
	SpecialityGid      int         `json:"SpecialityGid"`
	SpecialityOid      int         `json:"SpecialityOid"`
	YearOfEducation    int         `json:"YearOfEducation"`
	Amount             int         `json:"amount"`
	ChairGid           int         `json:"chairGid"`
	ChairOid           int         `json:"chairOid"`
	Course             int         `json:"course"`
	Faculty            string      `json:"faculty"`
	FacultyGid         int         `json:"facultyGid"`
	FacultyOid         int         `json:"facultyOid"`
	FormOfEducation    string      `json:"formOfEducation"`
	GroupGUID          string      `json:"groupGUID"`
	GroupGid           int64       `json:"groupGid"`
	GroupOid           int         `json:"groupOid"`
	GroupUID           interface{} `json:"groupUID"`
	KindEducation      int         `json:"kindEducation"`
	Name               string      `json:"name"`
	Number             string      `json:"number"`
	Plannedamount      int         `json:"plannedamount"`
	Speciality         string      `json:"speciality"`
}

type Lesson struct {
	Auditorium                string `json:"auditorium"`
	AuditoriumAmount          int    `json:"auditoriumAmount"`
	AuditoriumGUID            string `json:"auditoriumGUID"`
	AuditoriumOid             int    `json:"auditoriumOid"`
	Author                    string `json:"author"`
	BeginLesson               string `json:"beginLesson"`
	Building                  string `json:"building"`
	BuildingGid               int64  `json:"buildingGid"`
	BuildingOid               int    `json:"buildingOid"`
	ContentOfLoadOid          int    `json:"contentOfLoadOid"`
	ContentOfLoadUID          any    `json:"contentOfLoadUID"`
	ContentTableOfLessonsName string `json:"contentTableOfLessonsName"`
	ContentTableOfLessonsOid  int    `json:"contentTableOfLessonsOid"`
	Createddate               string `json:"createddate"`
	Date                      string `json:"date"`
	DateOfNest                string `json:"dateOfNest"`
	DayOfWeek                 int    `json:"dayOfWeek"`
	DayOfWeekString           string `json:"dayOfWeekString"`
	DetailInfo                string `json:"detailInfo"`
	Discipline                string `json:"discipline"`
	DisciplineOid             int    `json:"disciplineOid"`
	Disciplineinplan          any    `json:"disciplineinplan"`
	Disciplinetypeload        int    `json:"disciplinetypeload"`
	Duration                  int    `json:"duration"`
	EndLesson                 string `json:"endLesson"`
	Group                     any    `json:"group"`
	GroupGUID                 any    `json:"groupGUID"`
	GroupOid                  int    `json:"groupOid"`
	GroupUID                  any    `json:"groupUID"`
	GroupFacultyname          any    `json:"group_facultyname"`
	GroupFacultyoid           int    `json:"group_facultyoid"`
	Hideincapacity            int    `json:"hideincapacity"`
	IsBan                     bool   `json:"isBan"`
	KindOfWork                string `json:"kindOfWork"`
	KindOfWorkComplexity      int    `json:"kindOfWorkComplexity"`
	KindOfWorkOid             int    `json:"kindOfWorkOid"`
	KindOfWorkUID             any    `json:"kindOfWorkUid"`
	Lecturer                  string `json:"lecturer"`
	LecturerCustomUID         any    `json:"lecturerCustomUID"`
	LecturerEmail             string `json:"lecturerEmail"`
	LecturerGUID              string `json:"lecturerGUID"`
	LecturerOid               int    `json:"lecturerOid"`
	LecturerUID               string `json:"lecturerUID"`
	LecturerPostOid           int    `json:"lecturer_post_oid"`
	LecturerRank              string `json:"lecturer_rank"`
	LecturerTitle             string `json:"lecturer_title"`
	LessonNumberEnd           int    `json:"lessonNumberEnd"`
	LessonNumberStart         int    `json:"lessonNumberStart"`
	LessonOid                 int    `json:"lessonOid"`
	ListGroups                []any  `json:"listGroups"`
	ListOfLecturers           []struct {
		Lecturer          string `json:"lecturer"`
		LecturerCustomUID any    `json:"lecturerCustomUID"`
		LecturerEmail     string `json:"lecturerEmail"`
		LecturerGUID      any    `json:"lecturerGUID"`
		LecturerOid       int    `json:"lecturerOid"`
		LecturerUID       string `json:"lecturerUID"`
		LecturerPostOid   int    `json:"lecturer_post_oid"`
		LecturerRank      string `json:"lecturer_rank"`
		LecturerTitle     string `json:"lecturer_title"`
	} `json:"listOfLecturers"`
	Modifieddate       string `json:"modifieddate"`
	Note               any    `json:"note"`
	NoteDescription    string `json:"note_description"`
	Parentschedule     string `json:"parentschedule"`
	Replaces           any    `json:"replaces"`
	Stream             string `json:"stream"`
	StreamOid          int    `json:"streamOid"`
	StreamFacultyoid   int    `json:"stream_facultyoid"`
	SubGroup           string `json:"subGroup"`
	SubGroupOid        int    `json:"subGroupOid"`
	SubgroupFacultyoid int    `json:"subgroup_facultyoid"`
	TableofLessonsName string `json:"tableofLessonsName"`
	TableofLessonsOid  int    `json:"tableofLessonsOid"`
	URL1               string `json:"url1"`
	URL1Description    any    `json:"url1_description"`
	URL2               any    `json:"url2"`
	URL2Description    any    `json:"url2_description"`
}

type Config struct {
	UPDATE_CONFIG_TIMEOUT int `json:"UPDATE_CONFIG_TIMEOUT"`
}

//time function

func todayDate() string {
	today := time.Now()
	return today.Format("01-02-2006")
}

func tomorrowDate() string {
	today := time.Now()
	tomorrow := today.AddDate(0, 0, 1)
	return tomorrow.Format("01-02-2006")
}
