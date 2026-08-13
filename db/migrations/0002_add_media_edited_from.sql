-- AlterTable
ALTER TABLE `Media` ADD COLUMN `editedFromId` VARCHAR(191) NULL;

-- AddForeignKey
ALTER TABLE `Media` ADD CONSTRAINT `Media_editedFromId_fkey` FOREIGN KEY (`editedFromId`) REFERENCES `Media`(`id`) ON DELETE SET NULL ON UPDATE CASCADE;
