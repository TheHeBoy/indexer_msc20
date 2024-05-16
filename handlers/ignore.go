package handlers

/**
 * Special Note
 * Because there was a calculation bug in the indexer that caused some transactions to be judged as illegal,
 * in order to ensure the consistency of the indexing results, the following transactions will be added to the ignore list,
 * and this behavior will not lead to the loss of user assets.
 */
var ignoreHashes = map[string]bool{
	//"0x6f90e3494cd4db61cdb283a9d130aa9a37840051f6919efcd700385367937d90": true,
}
