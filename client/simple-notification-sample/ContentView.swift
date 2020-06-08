//
//  ContentView.swift
//  simple-notification-sample
//
//  Created by fino on 2020/06/08.
//  Copyright © 2020 yaegaki. All rights reserved.
//

import SwiftUI
import Firebase

struct ContentView: View {
    var body: some View {
        Button(action: {
            Messaging.messaging().subscribe(toTopic: "sample") { error in
                print("Subscribed!")
            }
        }) {
            Text("Subscribe")
        }
    }
}

struct ContentView_Previews: PreviewProvider {
    static var previews: some View {
        ContentView()
    }
}
