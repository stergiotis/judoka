nb=$ScriptCommandLine[[-1]];If[
FileExistsQ[nb],ResourceFunction["SaveReadableNotebook"][nb,
   StringReplace[nb, RegularExpression["%s"] -> "%s"],
   "ExcludedCellOptions" -> {CellChangeTimes,ExpressionUUID},
   "ExcludedNotebookOptions" -> {WindowSize, WindowMargins, ScrollingOptions},
   CharacterEncoding -> "UTF8",
   "IndentSize" -> 3]];