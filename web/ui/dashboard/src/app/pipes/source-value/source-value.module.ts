import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { SourceValuePipe } from './source-value.pipe';

@NgModule({
	declarations: [SourceValuePipe],
	imports: [CommonModule],
	exports: [SourceValuePipe]
})
export class SourceValueModule {}
