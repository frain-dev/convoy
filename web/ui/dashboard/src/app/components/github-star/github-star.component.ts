import { Component, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';

@Component({
  selector: 'convoy-github-star',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './github-star.component.html',
  styleUrls: ['./github-star.component.scss']
})
export class GithubStarComponent implements OnInit {

  constructor() { }

  ngOnInit(): void {
  }

}
